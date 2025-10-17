import { Injectable, OnModuleInit, Logger } from '@nestjs/common';
import { Cron, CronExpression } from '@nestjs/schedule';
import { RedisService } from '../redis/redis.service';
import { PrismaService } from '../tenants/prisma.service';
import { MeterGateConfigService } from '../config/metergate-config.service';

@Injectable()
export class CacheSyncService implements OnModuleInit {
  private readonly logger = new Logger(CacheSyncService.name);

  constructor(
    private readonly redis: RedisService,
    private readonly prisma: PrismaService,
    private readonly config: MeterGateConfigService
  ) {}

  async onModuleInit(): Promise<void> {
    // Perform an initial sync unconditionally on startup
    // (This guarantees the cache is warm before serving traffic)
    this.logger.log('Performing initial DB-to-Redis sync on startup...');
    await this.performSync();
  }

  @Cron(CronExpression.EVERY_10_SECONDS)
  async handleCron(): Promise<void> {
    // Leader Election using Redis SET NX EX
    // TTL is 5 seconds so if the node dies, the lock is freed quickly.
    const lockKey = 'metergate:sync:lock';
    const lockAcquired = await this.redis.client.set(lockKey, 'locked', 'EX', 5, 'NX');

    if (lockAcquired !== 'OK') {
      // Failed to acquire lock, meaning another instance is currently syncing.
      return;
    }

    this.logger.debug('Lock acquired: Executing background DB-to-Redis sync...');
    try {
      await this.performSync();
    } catch (error) {
      this.logger.error('Failed to execute background sync', error);
    }
  }

  private async performSync(): Promise<void> {
    const conf = this.config.get();

    // 1. Publish static blacklist state to Redis L2 set
    if (conf.blacklist.staticApiKeyHashes.length > 0) {
      const pipeline = this.redis.client.pipeline();
      pipeline.sadd('metergate:blacklist:api-key-hashes', ...conf.blacklist.staticApiKeyHashes);
      await pipeline.exec();
    }

    // 2. Publish plan configuration (including shardCount) to Redis L2
    for (const [planCode, plan] of Object.entries(conf.plans)) {
      await this.redis.setJson(`metergate:plan:${planCode}`, { shardCount: plan.shardCount }, conf.cache.l2.tenantPlanTtlSeconds);
    }

    // 3. Publish all authoritative API keys from DB to Redis L2
    const allKeys = await this.prisma.apiKey.findMany({ include: { tenant: true } });
    for (const key of allKeys) {
      const cacheKey = `metergate:cache:apikey:${key.keyHash}`;
      const value = {
        tenantId: key.tenant.id,
        tenantSlug: key.tenant.slug,
        planCode: key.tenant.planCode,
        revoked: key.revokedAt !== null
      };
      await this.redis.setJson(cacheKey, value, conf.cache.l2.apiKeyTtlSeconds);
    }
  }
}
