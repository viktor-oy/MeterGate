import { Test, TestingModule } from '@nestjs/testing';
import { TenantController } from '../../src/controlplane/tenants/tenant.controller';
import { PrismaService } from '../../src/controlplane/tenants/prisma.service';
import { HttpStatus } from '@nestjs/common';

describe('TenantController', () => {
  let controller: TenantController;
  let prisma: jest.Mocked<PrismaService>;

  beforeEach(async () => {
    const prismaMock = {
      tenant: {
        findUnique: jest.fn(),
        create: jest.fn(),
      },
    };

    const module: TestingModule = await Test.createTestingModule({
      controllers: [TenantController],
      providers: [{ provide: PrismaService, useValue: prismaMock }],
    }).compile();

    controller = module.get<TenantController>(TenantController);
    prisma = module.get(PrismaService) as unknown as jest.Mocked<PrismaService>;
  });

  it('should create a tenant', async () => {
    (prisma.tenant.findUnique as jest.Mock).mockResolvedValue(null);
    (prisma.tenant.create as jest.Mock).mockResolvedValue({
      id: 'uuid',
      slug: 'test',
      name: 'Test Tenant',
      planCode: 'free',
      createdAt: new Date(),
    });

    const result = await controller.create({ slug: 'test', name: 'Test Tenant', planCode: 'free' });
    expect(result).toHaveProperty('id', 'uuid');
    expect(prisma.tenant.create).toHaveBeenCalled();
  });

  it('should throw conflict if slug exists', async () => {
    (prisma.tenant.findUnique as jest.Mock).mockResolvedValue({ id: 'uuid', slug: 'test', name: 'Test', planCode: 'free', createdAt: new Date() });
    await expect(controller.create({ slug: 'test', name: 'Test', planCode: 'free' })).rejects.toThrow();
  });
});
