import { Injectable, OnModuleDestroy } from '@nestjs/common';
import Redis from 'ioredis';

const fixedWindowLua = `
local current = redis.call("INCR", KEYS[1])

if current == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[2])
end

local limit = tonumber(ARGV[1])
local ttl = redis.call("TTL", KEYS[1])

if current > limit then
  return {0, current, 0, ttl}
end

return {1, current, limit - current, ttl}
`;

export type RateLimitResult = {
  allowed: boolean;
  current: number;
  remaining: number;
  resetSeconds: number;
};

@Injectable()
export class RedisService implements OnModuleDestroy {
  private readonly client: Redis;

  constructor() {
    this.client = new Redis(process.env.REDIS_URL ?? 'redis://localhost:6379', {
      maxRetriesPerRequest: 2,
      lazyConnect: true
    });
  }

  async onModuleDestroy(): Promise<void> {
    this.client.disconnect();
  }

  async fixedWindow(key: string, limit: number, ttlSeconds: number): Promise<RateLimitResult> {
    const response = (await this.client.eval(fixedWindowLua, 1, key, String(limit), String(ttlSeconds))) as [
      number,
      number,
      number,
      number
    ];

    return {
      allowed: response[0] === 1,
      current: response[1],
      remaining: response[2],
      resetSeconds: Math.max(response[3], 0)
    };
  }

  async isDynamicallyBlacklisted(keyHash: string): Promise<boolean> {
    return (await this.client.sismember('metergate:blacklist:api-key-hashes', keyHash)) === 1;
  }

  async getJson<T>(key: string): Promise<T | undefined> {
    const value = await this.client.get(key);
    return value ? (JSON.parse(value) as T) : undefined;
  }

  async setJson<T>(key: string, value: T, ttlSeconds: number): Promise<void> {
    await this.client.set(key, JSON.stringify(value), 'EX', ttlSeconds);
  }
}

