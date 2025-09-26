import { Injectable } from '@nestjs/common';
import { L1CacheService } from '../cache/l1-cache.service';
import { MeterGateConfigService } from '../config/metergate-config.service';
import type { ApiKeyRecord } from '../policy/policy.types';
import { RedisService } from '../redis/redis.service';
import { PrismaService } from './prisma.service';

@Injectable()
export class ApiKeyRepository {
  constructor(
    private readonly prisma: PrismaService,
    private readonly l1: L1CacheService,
    private readonly redis: RedisService,
    private readonly config: MeterGateConfigService
  ) {}

  async findByHash(keyHash: string): Promise<ApiKeyRecord | undefined> {
    const cacheKey = `apikey:${keyHash}`;
    const cached = this.l1.get(cacheKey) as ApiKeyRecord | undefined;
    if (cached) {
      return cached;
    }

    const l2 = (await this.redis.getJson(cacheKey)) as ApiKeyRecord | undefined;
    if (l2) {
      this.l1.set(cacheKey, l2, this.config.get().cache.l1.apiKeyTtlSeconds);
      return l2;
    }

    const record = await this.prisma.apiKey.findUnique({
      where: { keyHash },
      include: { tenant: true }
    });

    if (!record) {
      return undefined;
    }

    const value: ApiKeyRecord = {
      tenantId: record.tenant.id,
      tenantSlug: record.tenant.slug,
      planCode: record.tenant.planCode,
      revoked: record.revokedAt !== null
    };

    this.l1.set(cacheKey, value, this.config.get().cache.l1.apiKeyTtlSeconds);
    await this.redis.setJson(cacheKey, value, this.config.get().cache.l2.apiKeyTtlSeconds);
    return value;
  }
}
