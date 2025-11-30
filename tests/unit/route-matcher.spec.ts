import { L1CacheService } from '../../src/cache/l1-cache.service';
import { MeterGateConfigService } from '../../src/config/metergate-config.service';
import { RouteMatcherService } from '../../src/routes/route-matcher.service';

describe('RouteMatcherService', () => {
  it('matches by method and path prefix without regex routing', () => {
    process.env.METERGATE_CONFIG_PATH = 'config/metergate.yml';
    const config = new MeterGateConfigService();
    const matcher = new RouteMatcherService(config, new L1CacheService(config));

    expect(matcher.match('POST', '/graphql')?.id).toBe('graphql');
    expect(matcher.match('GET', '/graphql')).toBeUndefined();
    expect(matcher.match('POST', '/unprotected')).toBeUndefined();
  });
});

