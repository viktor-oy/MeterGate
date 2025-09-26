import { Resolver, Query } from '@nestjs/graphql';

/**
 * Placeholder GraphQL resolver that exposes a simple health query.
 * This serves as the seed resolver required by Apollo Server's schema-first
 * auto-generation mode (autoSchemaFile: true) — without at least one
 * @Query decorator, the schema builder throws a validation error.
 *
 * As Tenant, Plan, and API Key resolvers are introduced in subsequent stages,
 * this resolver will be extended or superseded.
 */
@Resolver()
export class HealthResolver {
  @Query(() => String, { description: 'Returns the Control Plane operational status.' })
  health(): string {
    return 'ok';
  }
}
