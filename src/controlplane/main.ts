import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { DocumentBuilder, SwaggerModule } from '@nestjs/swagger';
import { json } from 'express';
import { AppModule } from './app.module';
import { MeterGateConfigService } from './config/metergate-config.service';
import { StructuredLoggerService } from './observability/structured-logger.service';

type JsonResponse = {
  json: (body: unknown) => void;
};

type HttpAdapterWithGet = {
  get: (path: string, handler: (request: unknown, response: JsonResponse) => void) => void;
};

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
    .setTitle('MeterGate Control Plane API')
    .setDescription('OpenAPI REST documentation for MeterGate Management and Infrastructure Automation.')
    .setVersion('0.1.0')
    .addApiKey({ type: 'apiKey', name: config.getApiKeyHeader(), in: 'header' }, 'api-key')
    .build();
  const document = SwaggerModule.createDocument(app, documentConfig);
  // @nestjs/swagger is explicitly configured here for DevOps/IaC endpoints (/docs).
  // The Admin UI dashboard endpoint is served via @nestjs/graphql at /graphql (see app.module.ts).
  SwaggerModule.setup('/docs', app, document);
  const httpAdapter = app.getHttpAdapter() as HttpAdapterWithGet;
  httpAdapter.get('/docs-json', (_request, response) => {
    response.json(document);
  });

  const port = config.getPort();
  await app.listen(port, '0.0.0.0');
  logger.log(`MeterGate listening on ${String(port)}`, 'Bootstrap');
}

void bootstrap();
