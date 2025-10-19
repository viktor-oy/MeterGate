import { Injectable } from '@nestjs/common';
import { LRUCache } from 'lru-cache';
import { MeterGateConfigService } from '../config/metergate-config.service';

@Injectable()
export class L1CacheService {
  private readonly cache: LRUCache<string, unknown>;

  constructor(configService: MeterGateConfigService) {
    this.cache = new LRUCache({
      max: configService.get().cache.l1.maxItems,
      ttlAutopurge: true
    });
  }

  get<T>(key: string): T | undefined {
    return this.cache.get(key) as T | undefined;
  }

  set<T>(key: string, value: T, ttlSeconds: number): void {
    this.cache.set(key, value, { ttl: ttlSeconds * 1000 });
  }

  delete(key: string): void {
    this.cache.delete(key);
  }
}

