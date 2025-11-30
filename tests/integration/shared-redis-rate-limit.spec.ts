import { MeterGateConfigService } from '../../src/config/metergate-config.service';
import { RateLimitService } from '../../src/policy/rate-limit.service';

describe('shared Redis rate limits', () => {
  it('uses a replica-neutral counter key so scaled MeterGate instances share limits', async () => {
    process.env.METERGATE_CONFIG_PATH = 'config/metergate.yml';
    const calls: string[] = [];
    const redis = {
      fixedWindow: jest.fn(async (key: string) => {
        calls.push(key);
        return { allowed: true, current: calls.length, remaining: 6 - calls.length, resetSeconds: 60 };
      })
    };

    const config = new MeterGateConfigService();
    const firstReplica = new RateLimitService(config, redis as never);
    const secondReplica = new RateLimitService(config, redis as never);
    const route = { id: 'graphql', method: 'POST', pathPrefix: '/graphql', upstreamPath: '/graphql', planGroup: 'graphql' };

    await firstReplica.check('tenant-1', 'free', route);
    await secondReplica.check('tenant-1', 'free', route);

    expect(calls[0]).toBe(calls[1]);
  });
});

