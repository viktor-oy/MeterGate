export type PolicyReason =
  | 'ALLOWED'
  | 'UNPROTECTED_ROUTE'
  | 'API_KEY_REQUIRED'
  | 'API_KEY_INVALID'
  | 'API_KEY_REVOKED'
  | 'API_KEY_BLACKLISTED'
  | 'RATE_LIMIT_EXCEEDED';

export type PolicyDecision = {
  allowed: boolean;
  statusCode: 200 | 401 | 403 | 429;
  reason: PolicyReason;
  tenantId?: string;
  tenantSlug?: string;
  routeId?: string;
  planCode?: string;
  limit?: number;
  remaining?: number;
  resetSeconds?: number;
};

export type PolicyRequest = {
  method: string;
  path: string;
  apiKey?: string;
  requestId: string;
};

export type MatchedRoute = {
  id: string;
  method: string;
  pathPrefix: string;
  upstreamPath: string;
  planGroup: string;
};

export type ApiKeyRecord = {
  tenantId: string;
  tenantSlug: string;
  planCode: string;
  revoked: boolean;
};

