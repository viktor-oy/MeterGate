import { L1CacheService } from '../../src/controlplane/cache/l1-cache.service';
import { MeterGateConfigService } from '../../src/controlplane/config/metergate-config.service';
import { BlacklistService } from '../../src/controlplane/policy/blacklist.service';

describe('BlacklistService', () => {
  it('checks config blacklist before Redis dynamic state', async () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    const config = new MeterGateConfigService();
    const redis = { isDynamicallyBlacklisted: jest.fn(async () => false) };
    const service = new BlacklistService(config, redis as never, new L1CacheService(config));

    await expect(service.isBlacklisted('6a2dbb79944a67e8f6fe7ea3e96f6c9af8f43d897780d589740d925668aa3c83')).resolves.toBe(true);
    expect(redis.isDynamicallyBlacklisted).not.toHaveBeenCalled();
  });

  it('caches Redis blacklist membership briefly', async () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    const config = new MeterGateConfigService();
    const redis = { isDynamicallyBlacklisted: jest.fn(async () => true) };
    const service = new BlacklistService(config, redis as never, new L1CacheService(config));

    await service.isBlacklisted('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
    await service.isBlacklisted('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');

    expect(redis.isDynamicallyBlacklisted).toHaveBeenCalledTimes(1);
  });
});

