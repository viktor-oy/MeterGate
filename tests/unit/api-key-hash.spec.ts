import { ApiKeyHashService } from '../../src/keys/api-key-hash.service';

describe('ApiKeyHashService', () => {
  it('hashes keys without exposing plaintext', () => {
    process.env.METERGATE_KEY_HASH_SECRET = 'unit-secret';
    const service = new ApiKeyHashService();

    const hash = service.hash('mg_demo_local_plaintext_key');

    expect(hash).toMatch(/^[a-f0-9]{64}$/);
    expect(hash).not.toContain('plaintext');
    expect(service.equalsHash('mg_demo_local_plaintext_key', hash)).toBe(true);
  });
});

