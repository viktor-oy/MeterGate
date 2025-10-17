import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';
import { RateLimitService } from '../../src/controlplane/policy/rate-limit.service';

describe('RateLimitService', () => {
  it('uses tenant route and plan in distributed counter keys', async () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
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
    const [counterKey] = redis.fixedWindow.mock.calls[0] as unknown as [string, number, number];
    expect(counterKey).toContain('metergate:rate:tenant-1:graphql:free');
  });
});
