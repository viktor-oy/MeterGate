import { Injectable } from '@nestjs/common';
import { L1CacheService } from '../cache/l1-cache.service';
import { MeterGateConfigService } from '../config/metergate-config.service';
import { RedisService } from '../redis/redis.service';

@Injectable()
export class BlacklistService {
  constructor(
    private readonly config: MeterGateConfigService,
    private readonly redis: RedisService,
    private readonly l1: L1CacheService
  ) {}

  async isBlacklisted(keyHash: string): Promise<boolean> {
    const staticHashes = this.config.getStaticBlacklistHashes();
    if (staticHashes.has(keyHash.toLowerCase())) {
      return true;
    }

    const cacheKey = `metergate:cache:blacklist:${keyHash}`;
    const cached = this.l1.get(cacheKey) as boolean | undefined;
    if (cached !== undefined) {
      return cached;
    }

    const dynamic = await this.redis.isDynamicallyBlacklisted(keyHash);
    this.l1.set(cacheKey, dynamic, this.config.get().cache.l1.blacklistTtlSeconds);
    return dynamic;
  }
}
