import { Test, TestingModule } from '@nestjs/testing';
import { ApiKeyController } from '../../src/controlplane/keys/api-key.controller';
import { PrismaService } from '../../src/controlplane/tenants/prisma.service';
import { ApiKeyHashService } from '../../src/controlplane/keys/api-key-hash.service';
import { NotFoundException } from '@nestjs/common';

describe('ApiKeyController', () => {
  let controller: ApiKeyController;
  let prisma: jest.Mocked<PrismaService>;

  beforeEach(async () => {
    const prismaMock = {
      tenant: { findUnique: jest.fn() },
      apiKey: { create: jest.fn() },
    };

    const hashServiceMock = {
      hash: jest.fn().mockReturnValue('hashed_key'),
      prefix: jest.fn().mockReturnValue('mg_12345678')
    };

    const module: TestingModule = await Test.createTestingModule({
      controllers: [ApiKeyController],
      providers: [
        { provide: PrismaService, useValue: prismaMock },
        { provide: ApiKeyHashService, useValue: hashServiceMock },
      ],
    }).compile();

    controller = module.get<ApiKeyController>(ApiKeyController);
    prisma = module.get(PrismaService) as unknown as jest.Mocked<PrismaService>;
  });

  it('should create an API key', async () => {
    (prisma.tenant.findUnique as jest.Mock).mockResolvedValue({ id: 't1', slug: 'test', name: 'Test', planCode: 'free', createdAt: new Date() });
    (prisma.apiKey.create as jest.Mock).mockResolvedValue({
      id: 'k1',
      tenantId: 't1',
      name: 'Key',
      keyPrefix: 'mg_12345678',
      keyHash: 'hashed_key',
      revokedAt: null,
      createdAt: new Date(),
    });

    const result = await controller.create({ tenantId: 't1', name: 'Key' });
    expect(result).toHaveProperty('rawApiKey');
    expect(result.rawApiKey).toMatch(/^mg_[a-f0-9]{64}$/);
  });

  it('should throw if tenant not found', async () => {
    (prisma.tenant.findUnique as jest.Mock).mockResolvedValue(null);
    await expect(controller.create({ tenantId: 't1', name: 'Key' })).rejects.toThrow(NotFoundException);
  });
});
