import { Injectable } from '@nestjs/common';
import { L1CacheService } from '../cache/l1-cache.service';
import { MeterGateConfigService } from '../config/metergate-config.service';
import type { MatchedRoute } from '../policy/policy.types';

@Injectable()
export class RouteMatcherService {
  constructor(
    private readonly config: MeterGateConfigService,
    private readonly l1: L1CacheService
  ) {}

  match(method: string, path: string): MatchedRoute | undefined {
    const normalizedMethod = method.toUpperCase();
    const cacheKey = `route:${normalizedMethod}:${path}`;
    const cached = this.l1.get(cacheKey) as MatchedRoute | null | undefined;
    if (cached !== undefined) {
      return cached ?? undefined;
    }

    const route = this.config
      .get()
      .protectedRoutes.find(
        (candidate) =>
          candidate.method.toUpperCase() === normalizedMethod && path.startsWith(candidate.pathPrefix)
      );

    const result = route ? { ...route, method: route.method.toUpperCase() } : null;
    this.l1.set(cacheKey, result, this.config.get().cache.l1.routeTtlSeconds);
    return result ?? undefined;
  }
}
