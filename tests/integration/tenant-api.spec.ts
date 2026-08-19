import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import * as request from 'supertest';
import { AppModule } from '../../src/controlplane/app.module';
import { PrismaService } from '../../src/controlplane/tenants/prisma.service';

describe('Tenant API (e2e)', () => {
  let app: INestApplication;
  let prisma: PrismaService;

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
  });

  afterAll(async () => {
    await prisma.$disconnect();
    await app.close();
  });

  it('/tenants (POST) - creates a tenant', async () => {
    const payload = {
      slug: `test-slug-${Date.now()}`,
      name: 'Integration Test Tenant',
      planCode: 'starter'
    };

    const response = await request(app.getHttpServer())
      .post('/tenants')
      .send(payload)
      .expect(201);

    expect(response.body).toHaveProperty('id');
    expect(response.body.slug).toBe(payload.slug);

    // Verify in db
    const dbTenant = await prisma.tenant.findUnique({ where: { id: response.body.id } });
    expect(dbTenant).toBeDefined();
    
    // Cleanup
    await prisma.tenant.delete({ where: { id: response.body.id } });
  });

  it('/tenants (POST) - fails on duplicate slug', async () => {
    const payload = {
      slug: `dup-slug-${Date.now()}`,
      name: 'Duplicate Tenant',
      planCode: 'starter'
    };

    await request(app.getHttpServer()).post('/tenants').send(payload).expect(201);
    const res = await request(app.getHttpServer()).post('/tenants').send(payload).expect(409);
    
    expect(res.body.message).toBe('Tenant slug already exists');
    
    // Cleanup
    await prisma.tenant.delete({ where: { slug: payload.slug } });
  });
});
