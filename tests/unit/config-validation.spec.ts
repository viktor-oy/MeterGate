import { MeterGateConfigService } from '../../src/config/metergate-config.service';

describe('MeterGateConfigService', () => {
  it('loads a config with protected routes, plans, and blacklist state', () => {
    process.env.METERGATE_CONFIG_PATH = 'config/metergate.yml';
    const service = new MeterGateConfigService();

    expect(service.get().protectedRoutes).toHaveLength(2);
    expect(service.getPlanLimit('free', 'graphql')).toEqual({ limit: 6, unit: 'minute' });
    expect(service.getStaticBlacklistHashes().size).toBe(1);
  });
});

