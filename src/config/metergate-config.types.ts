export type RateUnit = 'second' | 'minute' | 'hour';

export type MeterGateConfig = {
  server: {
    port: number;
    apiKeyHeader: string;
  };
  proxy: {
    enabled: boolean;
    upstreamUrl: string;
    upstreamTimeoutMs: number;
  };
  cache: {
    l1: {
      maxItems: number;
      routeTtlSeconds: number;
      apiKeyTtlSeconds: number;
      tenantPlanTtlSeconds: number;
      blacklistTtlSeconds: number;
      configTtlSeconds: number;
    };
    l2: {
      apiKeyTtlSeconds: number;
      tenantPlanTtlSeconds: number;
    };
  };
  protectedRoutes: Array<{
    id: string;
    method: string;
    pathPrefix: string;
    upstreamPath: string;
    planGroup: string;
  }>;
  plans: Record<
    string,
    {
      displayName: string;
      groups: Record<
        string,
        {
          limit: number;
          unit: RateUnit;
        }
      >;
    }
  >;
  blacklist: {
    staticApiKeyHashes: string[];
  };
};

export type PlanLimit = {
  limit: number;
  unit: RateUnit;
};

