import { Injectable } from '@nestjs/common';
import { ApiKeyHashService } from '../keys/api-key-hash.service';
import { ApiKeyRepository } from '../tenants/api-key.repository';
import { RouteMatcherService } from '../routes/route-matcher.service';
import { BlacklistService } from './blacklist.service';
import type { PolicyDecision, PolicyRequest } from './policy.types';
import { RateLimitService } from './rate-limit.service';
import { OtelService } from '../observability/otel.service';
import { RedisService } from '../redis/redis.service';
import { MeterGateConfigService } from '../config/metergate-config.service';

@Injectable()
export class PolicyEngineService {
  constructor(
    private readonly routes: RouteMatcherService,
    private readonly keyHash: ApiKeyHashService,
    private readonly keys: ApiKeyRepository,
    private readonly blacklist: BlacklistService,
    private readonly rateLimit: RateLimitService,
    private readonly otel: OtelService,
    private readonly redis: RedisService,
    private readonly config: MeterGateConfigService
  ) {}

  async decide(request: PolicyRequest): Promise<PolicyDecision> {
    return this.otel.span('metergate.policy_decision', request.requestId, { path: request.path }, async () => {
      const route = this.routes.match(request.method, request.path);
      if (!route) {
        return { allowed: true, statusCode: 200, reason: 'UNPROTECTED_ROUTE' };
      }

      if (!request.apiKey) {
        return { allowed: false, statusCode: 401, reason: 'API_KEY_REQUIRED', routeId: route.id };
      }

      const hash = this.keyHash.hash(request.apiKey);
      if (await this.blacklist.isBlacklisted(hash)) {
        return { allowed: false, statusCode: 403, reason: 'API_KEY_BLACKLISTED', routeId: route.id };
      }

      const apiKey = await this.otel.span(
        'metergate.postgres_api_key_lookup',
        request.requestId,
        { keyPrefix: this.keyHash.prefix(request.apiKey) },
        () => this.keys.findByHash(hash)
      );
      if (!apiKey) {
        return { allowed: false, statusCode: 401, reason: 'API_KEY_INVALID', routeId: route.id };
      }
      if (apiKey.revoked) {
        return {
          allowed: false,
          statusCode: 403,
          reason: 'API_KEY_REVOKED',
          routeId: route.id,
          tenantId: apiKey.tenantId,
          tenantSlug: apiKey.tenantSlug,
          planCode: apiKey.planCode
        };
      }

      const rate = await this.otel.span(
        'metergate.redis_rate_limit_check',
        request.requestId,
        { tenantId: apiKey.tenantId, routeId: route.id },
        () => this.rateLimit.check(apiKey.tenantId, apiKey.planCode, route)
      );

      const common = {
        tenantId: apiKey.tenantId,
        tenantSlug: apiKey.tenantSlug,
        routeId: route.id,
        planCode: apiKey.planCode,
        limit: rate.limit,
        remaining: rate.remaining,
        resetSeconds: rate.resetSeconds
      };

      if (!rate.allowed) {
        return { allowed: false, statusCode: 429, reason: 'RATE_LIMIT_EXCEEDED', ...common };
      }

      return { allowed: true, statusCode: 200, reason: 'ALLOWED', ...common };
    });
  }
}

