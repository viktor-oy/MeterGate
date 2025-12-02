import { Injectable } from '@nestjs/common';
import { LRUCache } from 'lru-cache';
import { MeterGateConfigService } from '../config/metergate-config.service';

type CacheEntry = {
  value: unknown;
};

@Injectable()
export class L1CacheService {
  private readonly cache: LRUCache<string, CacheEntry>;

  constructor(configService: MeterGateConfigService) {
    this.cache = new LRUCache({
      max: configService.get().cache.l1.maxItems,
      ttlAutopurge: true
    });
  }

  get(key: string): unknown {
    return this.cache.get(key)?.value;
  }

  set(key: string, value: unknown, ttlSeconds: number): void {
    this.cache.set(key, { value }, { ttl: ttlSeconds * 1000 });
  }

  delete(key: string): void {
    this.cache.delete(key);
  }
}
