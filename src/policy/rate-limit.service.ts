import { Injectable } from '@nestjs/common';
import { MeterGateConfigService } from '../config/metergate-config.service';
import type { MatchedRoute } from './policy.types';
import { RedisService, type RateLimitResult } from '../redis/redis.service';

@Injectable()
export class RateLimitService {
  constructor(
    private readonly config: MeterGateConfigService,
    private readonly redis: RedisService
  ) {}

  async check(tenantId: string, planCode: string, route: MatchedRoute): Promise<RateLimitResult & { limit: number }> {
    const planLimit = this.config.getPlanLimit(planCode, route.planGroup);
    if (!planLimit) {
      throw new Error(`plan ${planCode} does not define group ${route.planGroup}`);
    }

    const ttlSeconds = this.config.ttlSeconds(planLimit.unit);
    const windowId = Math.floor(Date.now() / (ttlSeconds * 1000));
    const key = `metergate:rate:${tenantId}:${route.id}:${planCode}:${windowId}`;
    const result = await this.redis.fixedWindow(key, planLimit.limit, ttlSeconds);
    return { ...result, limit: planLimit.limit };
  }
}

