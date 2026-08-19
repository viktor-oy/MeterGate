import { MiddlewareConsumer, Module, NestModule } from '@nestjs/common';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriver, ApolloDriverConfig } from '@nestjs/apollo';
import { ScheduleModule } from '@nestjs/schedule';
import { MeterGateConfigService } from './config/metergate-config.service';
import { HealthController } from './http/health.controller';
import { HealthResolver } from './http/health.resolver';
import { RequestIdMiddleware } from './http/request-id.middleware';
import { ApiKeyHashService } from './keys/api-key-hash.service';
import { OtelService } from './observability/otel.service';
import { StructuredLoggerService } from './observability/structured-logger.service';
import { RedisService } from './redis/redis.service';
import { PrismaService } from './tenants/prisma.service';
import { CacheSyncService } from './sync/cache-sync.service';
import { TenantController } from './tenants/tenant.controller';
import { ApiKeyController } from './keys/api-key.controller';

@Module({
  controllers: [HealthController, TenantController, ApiKeyController],
  providers: [
    MeterGateConfigService,
    ApiKeyHashService,
    RedisService,
    PrismaService,
    StructuredLoggerService,
    OtelService,
    HealthResolver,
    CacheSyncService
  ],
  imports: [
    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloDriver,
      autoSchemaFile: true,
      playground: process.env.NODE_ENV !== 'production',
      path: '/graphql',
    }),
    ScheduleModule.forRoot()
  ]
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer): void {
    consumer.apply(RequestIdMiddleware).forRoutes('*');
  }
}
