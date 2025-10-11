import { createHash } from 'node:crypto';
import { PrismaClient } from '@prisma/client';

const prisma = new PrismaClient();

const demoApiKey = 'mg_demo_local_plaintext_key';
const secret = process.env.METERGATE_KEY_HASH_SECRET ?? 'local-demo-secret-change-me';

function hashApiKey(apiKey: string): string {
  return createHash('sha256').update(`${secret}:${apiKey}`).digest('hex');
}

async function main(): Promise<void> {
  const tenant = await prisma.tenant.upsert({
    where: { slug: 'acme-support' },
    update: { planCode: 'free' },
    create: {
      id: '4a9ca27d-9e50-4ec8-b6e6-1d2a612b4135',
      slug: 'acme-support',
      name: 'Acme Support',
      planCode: 'free'
    }
  });

  await prisma.apiKey.upsert({
    where: { keyHash: hashApiKey(demoApiKey) },
    update: { revokedAt: null, tenantId: tenant.id, keyPrefix: demoApiKey.slice(0, 8) },
    create: {
      id: 'f6a4274e-8702-4015-8f2c-fc51dbf9c7c1',
      tenantId: tenant.id,
      name: 'Local demo key',
      keyPrefix: demoApiKey.slice(0, 8),
      keyHash: hashApiKey(demoApiKey)
    }
  });
}

main()
  .then(async () => {
    await prisma.$disconnect();
  })
  .catch(async (error) => {
    console.error(error);
    await prisma.$disconnect();
    process.exit(1);
  });

