import { MeterGateConfigService } from '../../src/config/metergate-config.service';
import { RateLimitService } from '../../src/policy/rate-limit.service';

describe('RateLimitService', () => {
  it('uses tenant route and plan in distributed counter keys', async () => {
    process.env.METERGATE_CONFIG_PATH = 'config/metergate.yml';
    const redis = {
      fixedWindow: jest.fn(async () => ({ allowed: true, current: 1, remaining: 5, resetSeconds: 60 }))
    };
    const service = new RateLimitService(new MeterGateConfigService(), redis as never);

    const result = await service.check('tenant-1', 'free', {
      id: 'graphql',
      method: 'POST',
      pathPrefix: '/graphql',
      upstreamPath: '/graphql',
      planGroup: 'graphql'
    });

    expect(result.limit).toBe(6);
    expect(redis.fixedWindow.mock.calls[0][0]).toContain('metergate:rate:tenant-1:graphql:free');
  });
});

