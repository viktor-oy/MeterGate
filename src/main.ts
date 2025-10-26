import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import { json } from 'express';
import { AppModule } from './app.module';
import { MeterGateConfigService } from './config/metergate-config.service';
import { StructuredLoggerService } from './observability/structured-logger.service';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  const logger = app.get(StructuredLoggerService);
  const config = app.get(MeterGateConfigService);

  app.useLogger(logger);
  app.use(json({ limit: '1mb' }));
  app.enableCors({
    origin: true,
    allowedHeaders: ['content-type', 'x-api-key', 'x-request-id'],
    exposedHeaders: ['x-request-id', 'x-metergate-decision', 'x-ratelimit-limit', 'x-ratelimit-remaining', 'x-ratelimit-reset']
  });

  const documentConfig = new DocumentBuilder()
    .setTitle('MeterGate API')
    .setDescription('Provider-mode decision endpoint and service health for MeterGate.')
    .setVersion('0.1.0')
    .addApiKey({ type: 'apiKey', name: config.getApiKeyHeader(), in: 'header' }, 'api-key')
    .build();
  const document = SwaggerModule.createDocument(app, documentConfig);
  SwaggerModule.setup('/docs', app, document);
  app.getHttpAdapter().get('/docs-json', (_req, res) => res.json(document));

  await app.listen(config.getPort(), '0.0.0.0');
  logger.log(`MeterGate listening on ${config.getPort()}`, 'Bootstrap');
}

void bootstrap();

