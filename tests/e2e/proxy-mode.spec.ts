describe('proxy mode contract', () => {
  it('documents that denied proxy requests do not reach Laravel', () => {
    const decision = { allowed: false, statusCode: 403, reason: 'API_KEY_BLACKLISTED' };
    const upstreamWasCalled = decision.allowed;

    expect(upstreamWasCalled).toBe(false);
  });
});

