import { PolicyEngineService } from '../../src/controlplane/policy/policy-engine.service';

const span = async <T>(_name: string, _requestId: string, _attributes: Record<string, unknown>, operation: () => Promise<T>) =>
  operation();

describe('PolicyEngineService', () => {
  it('allows unprotected routes without an API key', async () => {
    const engine = new PolicyEngineService(
      { match: jest.fn(() => undefined) } as never,
      {} as never,
      {} as never,
      {} as never,
      {} as never,
      { span } as never,
      {} as never,
      {} as never
    );

    await expect(engine.decide({ method: 'GET', path: '/healthz', requestId: 'r1' })).resolves.toEqual({
      allowed: true,
      statusCode: 200,
      reason: 'UNPROTECTED_ROUTE'
    });
  });

  it('blocks revoked keys before rate limiting', async () => {
    const route = { id: 'graphql', method: 'POST', pathPrefix: '/graphql', upstreamPath: '/graphql', planGroup: 'graphql' };
    const rateLimit = { check: jest.fn() };
    const engine = new PolicyEngineService(
      { match: jest.fn(() => route) } as never,
      { hash: jest.fn(() => 'hash'), prefix: jest.fn(() => 'mg_demo_') } as never,
      { findByHash: jest.fn(async () => ({ tenantId: 't1', tenantSlug: 'acme', planCode: 'free', revoked: true })) } as never,
      { isBlacklisted: jest.fn(async () => false) } as never,
      rateLimit as never,
      { span } as never,
      {} as never,
      {} as never
    );

    const decision = await engine.decide({ method: 'POST', path: '/graphql', apiKey: 'k', requestId: 'r2' });

    expect(decision.reason).toBe('API_KEY_REVOKED');
    expect(rateLimit.check).not.toHaveBeenCalled();
  });

  it('returns rate-limit metadata on 429 decisions', async () => {
    const route = { id: 'graphql', method: 'POST', pathPrefix: '/graphql', upstreamPath: '/graphql', planGroup: 'graphql' };
    const engine = new PolicyEngineService(
      { match: jest.fn(() => route) } as never,
      { hash: jest.fn(() => 'hash'), prefix: jest.fn(() => 'mg_demo_') } as never,
      { findByHash: jest.fn(async () => ({ tenantId: 't1', tenantSlug: 'acme', planCode: 'free', revoked: false })) } as never,
      { isBlacklisted: jest.fn(async () => false) } as never,
      { check: jest.fn(async () => ({ allowed: false, current: 7, remaining: 0, resetSeconds: 42, limit: 6 })) } as never,
      { span } as never,
      {} as never,
      {} as never
    );

    await expect(engine.decide({ method: 'POST', path: '/graphql', apiKey: 'k', requestId: 'r3' })).resolves.toMatchObject({
      allowed: false,
      statusCode: 429,
      reason: 'RATE_LIMIT_EXCEEDED',
      limit: 6,
      remaining: 0,
      resetSeconds: 42
    });
  });
});

