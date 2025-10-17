import { L1CacheService } from '../../src/controlplane/cache/l1-cache.service';
import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';

describe('L1CacheService', () => {
  it('stores bounded ttl values', () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    const cache = new L1CacheService(new MeterGateConfigService());

    cache.set('tenant:1', { plan: 'free' }, 30);

    expect(cache.get('tenant:1')).toEqual({ plan: 'free' });
  });
});

