import { CacheSyncService } from '../../src/controlplane/sync/cache-sync.service';
import { PrismaService } from '../../src/controlplane/tenants/prisma.service';
import { RedisService } from '../../src/controlplane/redis/redis.service';
import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';

/**
 * Integration test: Verifies the Leader-Elected Background Cache Sync 
 * against live infrastructure. No mocks are used.
 *
 * Prerequisite: PostgreSQL and Redis must be running on localhost.
 */
describe('CacheSyncService integration', () => {
  let prisma: PrismaService;
  let redis: RedisService;
  let config: MeterGateConfigService;
  let syncService: CacheSyncService;

  beforeAll(async () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    process.env.DATABASE_URL = process.env.DATABASE_URL ?? 'postgresql://metergate:metergate@localhost:5432/metergate?schema=public';
    process.env.REDIS_URL = process.env.REDIS_URL ?? 'redis://localhost:6379';
    try {
      config = new MeterGateConfigService();
      redis = new RedisService();
      prisma = new PrismaService();
      await prisma.onModuleInit();
      syncService = new CacheSyncService(redis, prisma, config);
    } catch {
      // Infrastructure unavailable
    }
  });

  afterAll(async () => {
    if (redis) {
      redis.onModuleDestroy();
    }
    if (prisma) {
      await prisma.onModuleDestroy();
    }
  });

  it('syncs API keys from Postgres to Redis L2', async () => {
    if (!prisma || !redis) {
      return;
    }

    // 1. Insert a test tenant and API key into Postgres directly
    const tenant = await prisma.tenant.upsert({
      where: { slug: 'sync-intg-tenant' },
      update: {},
      create: { slug: 'sync-intg-tenant', name: 'Sync Test', planCode: 'free' }
    });
    
    await prisma.apiKey.upsert({
      where: { keyHash: 'sync_test_hash' },
      update: {},
      create: { keyHash: 'sync_test_hash', keyPrefix: 'sync', name: 'Test Key', tenantId: tenant.id }
    });

    // 2. Clear Redis cache just in case to ensure we are truly testing the sync
    const cacheKey = 'metergate:cache:apikey:sync_test_hash';
    await redis.client.del(cacheKey);

    let redisVal = await redis.getJson(cacheKey);
    expect(redisVal).toBeUndefined();

    // 3. Force the Cron job logic to run
    // We clear the lock first to guarantee it acts as the Leader
    await redis.client.del('metergate:sync:lock');
    await syncService.handleCron();

    // 4. Verify Redis L2 now has the authoritative state pushed from Postgres
    redisVal = await redis.getJson(cacheKey);
    expect(redisVal).toBeDefined();
    expect(redisVal).toMatchObject({
      tenantId: tenant.id,
      tenantSlug: 'sync-intg-tenant',
      planCode: 'free',
      revoked: false
    });
  });

  it('skips sync if another node holds the lock', async () => {
    if (!prisma || !redis) {
      return;
    }

    // 1. Simulate another node holding the lock
    await redis.client.set('metergate:sync:lock', 'locked', 'EX', 10);

    const tenant = await prisma.tenant.upsert({
      where: { slug: 'sync-skip-tenant' },
      update: {},
      create: { slug: 'sync-skip-tenant', name: 'Sync Skip Test', planCode: 'free' }
    });

    await prisma.apiKey.upsert({
      where: { keyHash: 'sync_skip_hash' },
      update: {},
      create: { keyHash: 'sync_skip_hash', keyPrefix: 'skip', name: 'Skip Key', tenantId: tenant.id }
    });

    const cacheKey = 'metergate:cache:apikey:sync_skip_hash';
    await redis.client.del(cacheKey);

    // 2. Run the cron handle
    await syncService.handleCron();

    // 3. It should NOT be in Redis because the sync was skipped (follower mode)
    const redisVal = await redis.getJson(cacheKey);
    expect(redisVal).toBeUndefined();
  });
});
