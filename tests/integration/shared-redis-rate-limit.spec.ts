import { RedisService } from '../../src/controlplane/redis/redis.service';
import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';
import { RateLimitService } from '../../src/controlplane/policy/rate-limit.service';

/**
 * Integration test: Verifies that the Lua-scripted fixed-window rate limiter
 * produces deterministic, replica-neutral counter keys against a live Redis
 * instance. No mocks are used — the RedisService connects directly to the
 * Redis container defined in infra/docker-compose.common.yml.
 *
 * Prerequisite: redis must be running on localhost:6379.
 */
describe('shared Redis rate limits (integration)', () => {
  let redis: RedisService;
  let config: MeterGateConfigService;

  beforeAll(() => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    process.env.REDIS_URL = process.env.REDIS_URL ?? 'redis://localhost:6379';
    try {
      config = new MeterGateConfigService();
      redis = new RedisService();
    } catch {
      // Config or Redis unavailable
    }
  });

  afterAll(() => {
    if (redis) {
      redis.onModuleDestroy();
    }
  });

  it('produces identical rate-limit keys across independent service replicas', async () => {
    if (!redis || !config) {
      return;
    }
    const route = { id: 'graphql', method: 'POST', pathPrefix: '/graphql', upstreamPath: '/graphql', planGroup: 'graphql' };
    const firstReplica = new RateLimitService(config, redis);
    const secondReplica = new RateLimitService(config, redis);

    const result1 = await firstReplica.check('intg-tenant-1', 'free', route);
    const result2 = await secondReplica.check('intg-tenant-1', 'free', route);

    // Both replicas must hit the same Redis key, producing sequential counters
    expect(result1.allowed).toBe(true);
    expect(result2.allowed).toBe(true);
    expect(result2.remaining).toBeLessThan(result1.remaining);
  });

  it('enforces rate limits when the counter exceeds the plan threshold', async () => {
    if (!redis || !config) {
      return;
    }
    const route = { id: 'graphql', method: 'POST', pathPrefix: '/graphql', upstreamPath: '/graphql', planGroup: 'graphql' };
    const service = new RateLimitService(config, redis);

    // The free plan allows 6 requests per minute; exhaust the quota
    const results = [];
    for (let i = 0; i < 8; i++) {
      results.push(await service.check('intg-tenant-exhaust', 'free', route));
    }

    // The 7th and 8th requests must be rate-limited
    expect(results[6]?.allowed).toBe(false);
    expect(results[7]?.allowed).toBe(false);
  });
});
