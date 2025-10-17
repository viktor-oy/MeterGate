import { createHash } from 'node:crypto';
import { PrismaClient } from '@prisma/client';
import * as fs from 'node:fs';
import * as path from 'node:path';

const prisma = new PrismaClient();

const secret = process.env.METERGATE_KEY_HASH_SECRET ?? 'local-demo-secret-change-me';

function hashApiKey(apiKey: string): string {
  return createHash('sha256').update(`${secret}:${apiKey}`).digest('hex');
}

async function main(): Promise<void> {
  // Ensure seeding is only executed in development/debug environments
  if (process.env.NODE_ENV === 'production') {
    console.log('Skipping database seed: Not in DEBUG/Development environment.');
    return;
  }

  const seedDataPath = path.resolve(__dirname, '../seed.json');
  const seedData = JSON.parse(fs.readFileSync(seedDataPath, 'utf8'));
  const demoApiKey = seedData.apiKey.plaintext;

  const tenant = await prisma.tenant.upsert({
    where: { slug: seedData.tenant.slug },
    update: { planCode: seedData.tenant.planCode },
    create: {
      id: seedData.tenant.id,
      slug: seedData.tenant.slug,
      name: seedData.tenant.name,
      planCode: seedData.tenant.planCode
    }
  });

  await prisma.apiKey.upsert({
    where: { keyHash: hashApiKey(demoApiKey) },
    update: { revokedAt: null, tenantId: tenant.id, keyPrefix: demoApiKey.slice(0, 8) },
    create: {
      id: seedData.apiKey.id,
      tenantId: tenant.id,
      name: seedData.apiKey.name,
      keyPrefix: demoApiKey.slice(0, 8),
      keyHash: hashApiKey(demoApiKey)
    }
  });

  console.log('Database seeding complete using seed.json.');
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

