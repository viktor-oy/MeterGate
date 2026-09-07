import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';

describe('MeterGateConfigService', () => {
  it('loads a config with protected routes and plans', () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    const service = new MeterGateConfigService();

    expect(service.get().protectedRoutes).toHaveLength(2);
    expect(service.getPlanLimit('free', 'graphql')).toEqual({ limit: 6, unit: 'minute' });

  });
});

