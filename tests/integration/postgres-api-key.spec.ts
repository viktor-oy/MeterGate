import { ApiKeyRepository } from '../../src/tenants/api-key.repository';

describe('ApiKeyRepository', () => {
  it('maps PostgreSQL API key rows into policy records and caches them', async () => {
    const prisma = {
      apiKey: {
        findUnique: jest.fn(async () => ({
          revokedAt: null,
          tenant: { id: 'tenant-1', slug: 'acme-support', planCode: 'free' }
        }))
      }
    };
    const l1 = {
      get: jest.fn(() => undefined),
      set: jest.fn()
    };
    const redis = {
      getJson: jest.fn(async () => undefined),
      setJson: jest.fn(async () => undefined)
    };
    const config = {
      get: () => ({ cache: { l1: { apiKeyTtlSeconds: 30 }, l2: { apiKeyTtlSeconds: 30 } } })
    };

    const repo = new ApiKeyRepository(prisma as never, l1 as never, redis as never, config as never);

    await expect(repo.findByHash('hash')).resolves.toEqual({
      tenantId: 'tenant-1',
      tenantSlug: 'acme-support',
      planCode: 'free',
      revoked: false
    });
    expect(redis.setJson).toHaveBeenCalled();
  });
});

