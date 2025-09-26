import { createHash, timingSafeEqual } from 'node:crypto';
import { Injectable } from '@nestjs/common';

@Injectable()
export class ApiKeyHashService {
  private readonly secret = process.env.METERGATE_KEY_HASH_SECRET ?? 'local-demo-secret-change-me';

  hash(apiKey: string): string {
    return createHash('sha256').update(`${this.secret}:${apiKey}`).digest('hex');
  }

  prefix(apiKey: string): string {
    return apiKey.slice(0, 8);
  }

  equalsHash(apiKey: string, expectedHash: string): boolean {
    const actual = Buffer.from(this.hash(apiKey), 'hex');
    const expected = Buffer.from(expectedHash, 'hex');
    return actual.length === expected.length && timingSafeEqual(actual, expected);
  }
}

