import { readFileSync } from 'node:fs';
import { Injectable } from '@nestjs/common';
import { load } from 'js-yaml';
import { z } from 'zod';
import type { MeterGateConfig, PlanLimit } from './metergate-config.types';

const rateUnitSchema = z.enum(['second', 'minute', 'hour']);

const configSchema = z.object({
  server: z.object({
    port: z.number().int().positive(),
    apiKeyHeader: z.string().min(1)
  }),
  proxy: z.object({
    upstreamUrl: z.url(),
    upstreamTimeoutMs: z.number().int().positive()
  }),
  cache: z.object({
    l1: z.object({
      maxItems: z.number().int().positive(),
      routeTtlSeconds: z.number().int().positive(),
      apiKeyTtlSeconds: z.number().int().positive(),
      tenantPlanTtlSeconds: z.number().int().positive(),
      configTtlSeconds: z.number().int().positive()
    }),
    l2: z.object({
      apiKeyTtlSeconds: z.number().int().positive(),
      tenantPlanTtlSeconds: z.number().int().positive()
    })
  }),
  protectedRoutes: z
    .array(
      z.object({
        id: z.string().min(1),
        method: z.string().min(1),
        pathPrefix: z.string().startsWith('/'),
        upstreamPath: z.string().startsWith('/'),
        planGroup: z.string().min(1)
      })
    )
    .min(1),
  plans: z.record(
    z.string(),
    z.object({
      displayName: z.string().min(1),
      shardCount: z.number().int().positive().default(1),
      groups: z.record(
        z.string(),
        z.object({
          limit: z.number().int().positive(),
          unit: rateUnitSchema
        })
      )
    })
  )
});

@Injectable()
export class MeterGateConfigService {
  private readonly config: MeterGateConfig;

  constructor() {
    const path = process.env.METERGATE_CONFIG_PATH ?? 'infra/metergate.yml';
    const raw = load(readFileSync(path, 'utf8'));
    this.config = configSchema.parse(raw) as MeterGateConfig;
  }

  get(): MeterGateConfig {
    return this.config;
  }

  getPort(): number {
    return Number(process.env.METERGATE_PORT ?? this.config.server.port);
  }

  getApiKeyHeader(): string {
    return (process.env.METERGATE_API_KEY_HEADER ?? this.config.server.apiKeyHeader).toLowerCase();
  }

  getPlanLimit(planCode: string, group: string): PlanLimit | undefined {
    return this.config.plans[planCode]?.groups[group];
  }

  ttlSeconds(unit: PlanLimit['unit']): number {
    if (unit === 'second') {
      return 1;
    }
    if (unit === 'minute') {
      return 60;
    }
    return 3600;
  }
}
