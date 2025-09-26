import { MiddlewareConsumer, Module, NestModule } from '@nestjs/common';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriver, ApolloDriverConfig } from '@nestjs/apollo';
import { L1CacheService } from './cache/l1-cache.service';
import { MeterGateConfigService } from './config/metergate-config.service';
import { HealthController } from './http/health.controller';
import { ProviderController } from './http/provider.controller';
import { RequestIdMiddleware } from './http/request-id.middleware';
import { ApiKeyHashService } from './keys/api-key-hash.service';
import { OtelService } from './observability/otel.service';
import { StructuredLoggerService } from './observability/structured-logger.service';
import { BlacklistService } from './policy/blacklist.service';
import { PolicyEngineService } from './policy/policy-engine.service';
import { RateLimitService } from './policy/rate-limit.service';
import { ProxyController } from './proxy/proxy.controller';
import { RedisService } from './redis/redis.service';
import { RouteMatcherService } from './routes/route-matcher.service';
import { ApiKeyRepository } from './tenants/api-key.repository';
import { PrismaService } from './tenants/prisma.service';

@Module({
  controllers: [HealthController, ProviderController, ProxyController],
  providers: [
    MeterGateConfigService,
    L1CacheService,
    ApiKeyHashService,
    RedisService,
    PrismaService,
    ApiKeyRepository,
    RouteMatcherService,
    BlacklistService,
    RateLimitService,
    PolicyEngineService,
    StructuredLoggerService,
    OtelService
  ],
  imports: [
    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloDriver,
      autoSchemaFile: true,
      playground: true,
      path: '/graphql',
    }),
  ]
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer): void {
    consumer.apply(RequestIdMiddleware).forRoutes('*');
  }
}

