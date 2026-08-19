import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
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
  app.useGlobalPipes(new ValidationPipe({ whitelist: true }));
  app.use(json({ limit: '1mb' }));
  app.enableCors({
    origin: true,
    allowedHeaders: ['content-type', 'x-api-key', 'x-request-id'],
    exposedHeaders: ['x-request-id', 'x-metergate-decision', 'x-ratelimit-limit', 'x-ratelimit-remaining', 'x-ratelimit-reset']
  });

  if (process.env.NODE_ENV !== 'production') {
    const documentConfig = new DocumentBuilder()
      .setTitle('MeterGate API')
      .setDescription('Administrative API for managing MeterGate configuration.')
      .setVersion('0.1.0')
      .addApiKey({ type: 'apiKey', name: config.getApiKeyHeader(), in: 'header' }, 'api-key')
      .build();
    const document = SwaggerModule.createDocument(app, documentConfig);
    SwaggerModule.setup('/docs', app, document);
    const httpAdapter = app.getHttpAdapter() as HttpAdapterWithGet;
    httpAdapter.get('/docs-json', (_request, response) => {
      response.json(document);
    });
  }

  const port = config.getPort();
  await app.listen(port, '0.0.0.0');
  logger.log(`MeterGate listening on ${String(port)}`, 'Bootstrap');
}

void bootstrap();
