import { Injectable, OnModuleDestroy } from '@nestjs/common';
import Redis, { Cluster } from 'ioredis';



@Injectable()
export class RedisService implements OnModuleDestroy {
  public readonly client: Redis | Cluster;

  constructor() {
    const urls = (process.env.REDIS_URL ?? 'redis://localhost:6379').split(',');
    
    if (urls.length > 1) {
      this.client = new Redis.Cluster(urls, {
        redisOptions: { maxRetriesPerRequest: 2 }
      });
    } else {
      this.client = new Redis(urls[0] as string, {
        maxRetriesPerRequest: 2,
        lazyConnect: true
      });
    }
  }

  onModuleDestroy(): void {
    this.client.disconnect();
  }



  async getJson(key: string): Promise<unknown> {
    const value = await this.client.get(key);
    return value ? (JSON.parse(value) as unknown) : undefined;
  }

  async setJson(key: string, value: unknown, ttlSeconds: number): Promise<void> {
    await this.client.set(key, JSON.stringify(value), 'EX', ttlSeconds);
  }
}
