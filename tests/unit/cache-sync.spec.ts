import { CacheSyncService } from '../../src/controlplane/sync/cache-sync.service';

describe('CacheSyncService (Leader Election)', () => {
  it('runs sync when lock is acquired (Leader)', async () => {
    const redisMock = {
      client: {
        set: jest.fn().mockResolvedValue('OK'),
        pipeline: jest.fn().mockReturnValue({
          sadd: jest.fn(),
          exec: jest.fn().mockResolvedValue(null)
        })
      },
      setJson: jest.fn().mockResolvedValue(null)
    } as any;

    const prismaMock = {
      apiKey: {
        findMany: jest.fn().mockResolvedValue([])
      }
    } as any;

    const configMock = {
      get: jest.fn().mockReturnValue({
        blacklist: { staticApiKeyHashes: [] },
        plans: {},
        cache: { l2: { apiKeyTtlSeconds: 60, tenantPlanTtlSeconds: 60 } }
      })
    } as any;

    const service = new CacheSyncService(redisMock, prismaMock, configMock);
    
    await service.handleCron();

    expect(redisMock.client.set).toHaveBeenCalledWith('metergate:sync:lock', 'locked', 'EX', 5, 'NX');
    expect(prismaMock.apiKey.findMany).toHaveBeenCalled();
  });

  it('skips sync when lock is denied (Follower)', async () => {
    const redisMock = {
      client: {
        set: jest.fn().mockResolvedValue(null),
      }
    } as any;

    const prismaMock = {
      apiKey: {
        findMany: jest.fn()
      }
    } as any;

    const configMock = {
      get: jest.fn()
    } as any;

    const service = new CacheSyncService(redisMock, prismaMock, configMock);
    
    await service.handleCron();

    expect(redisMock.client.set).toHaveBeenCalledWith('metergate:sync:lock', 'locked', 'EX', 5, 'NX');
    expect(prismaMock.apiKey.findMany).not.toHaveBeenCalled();
    expect(configMock.get).not.toHaveBeenCalled();
  });
});
