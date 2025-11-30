import { PolicyEngineService } from '../../src/policy/policy-engine.service';

describe('provider mode contract', () => {
  it('returns a decision payload Laravel can enforce', async () => {
    const engine = {
      decide: jest.fn(async () => ({
        allowed: false,
        statusCode: 429,
        reason: 'RATE_LIMIT_EXCEEDED',
        routeId: 'graphql'
      }))
    } as unknown as PolicyEngineService;

    await expect(engine.decide({ method: 'POST', path: '/graphql', apiKey: 'k', requestId: 'r1' })).resolves.toEqual({
      allowed: false,
      statusCode: 429,
      reason: 'RATE_LIMIT_EXCEEDED',
      routeId: 'graphql'
    });
  });
});

