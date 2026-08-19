import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import * as request from 'supertest';
import { AppModule } from '../../src/controlplane/app.module';
import { PrismaService } from '../../src/controlplane/tenants/prisma.service';

describe('ApiKey API (e2e)', () => {
  let app: INestApplication;
  let prisma: PrismaService;
  let testTenantId: string;

  beforeAll(async () => {
    process.env.METERGATE_CONFIG_PATH = 'infra/metergate.yml';
    process.env.DATABASE_URL = process.env.DATABASE_URL ?? 'postgresql://metergate:metergate@localhost:5432/metergate?schema=public';

    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ whitelist: true }));
    await app.init();
    
    prisma = app.get(PrismaService);

    // Create a tenant for the key tests
    const tenant = await prisma.tenant.create({
      data: {
        slug: `key-test-${Date.now()}`,
        name: 'Key Test Tenant',
        planCode: 'starter'
      }
    });
    testTenantId = tenant.id;
  });

  afterAll(async () => {
    await prisma.tenant.delete({ where: { id: testTenantId } });
    await prisma.$disconnect();
    await app.close();
  });

  it('/api-keys (POST) - creates an API key', async () => {
    const payload = {
      tenantId: testTenantId,
      name: 'Test Key'
    };

    const response = await request(app.getHttpServer())
      .post('/api-keys')
      .send(payload)
      .expect(201);

    expect(response.body).toHaveProperty('id');
    expect(response.body).toHaveProperty('rawApiKey');
    expect(response.body.rawApiKey).toMatch(/^mg_[a-f0-9]{64}$/);
    expect(response.body.tenantId).toBe(testTenantId);

    // Verify in db
    const dbKey = await prisma.apiKey.findUnique({ where: { id: response.body.id } });
    expect(dbKey).toBeDefined();
    expect(dbKey?.keyPrefix).toBe(response.body.rawApiKey.substring(0, 8));
  });

  it('/api-keys (POST) - fails for invalid tenant', async () => {
    const payload = {
      tenantId: '00000000-0000-0000-0000-000000000000',
      name: 'Test Key'
    };

    await request(app.getHttpServer())
      .post('/api-keys')
      .send(payload)
      .expect(404);
  });
});
