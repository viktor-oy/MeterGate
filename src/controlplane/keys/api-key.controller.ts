import { Controller, Post, Body, HttpCode, HttpStatus, NotFoundException } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse } from '@nestjs/swagger';
import { randomBytes } from 'crypto';
import { PrismaService } from '../tenants/prisma.service';
import { ApiKeyHashService } from './api-key-hash.service';
import { CreateApiKeyDto } from './dto/create-api-key.dto';

@ApiTags('API Keys')
@Controller('api-keys')
export class ApiKeyController {
  constructor(
    private readonly prisma: PrismaService,
    private readonly hashService: ApiKeyHashService
  ) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  @ApiOperation({ summary: 'Provision a new API Key for a Tenant' })
  @ApiResponse({ status: 201, description: 'API Key successfully created (raw key returned only once)' })
  @ApiResponse({ status: 404, description: 'Tenant not found' })
  async create(@Body() createApiKeyDto: CreateApiKeyDto) {
    const tenant = await this.prisma.tenant.findUnique({
      where: { id: createApiKeyDto.tenantId }
    });

    if (!tenant) {
      throw new NotFoundException('Tenant not found');
    }

    // Generate a secure API Key
    const rawKey = randomBytes(32).toString('hex');
    const fullKey = `mg_${rawKey}`;
    
    // Hash the key using our existing service
    const hashedKey = this.hashService.hash(fullKey);
    const prefix = this.hashService.prefix(fullKey);

    const apiKey = await this.prisma.apiKey.create({
      data: {
        tenantId: tenant.id,
        name: createApiKeyDto.name,
        keyPrefix: prefix,
        keyHash: hashedKey
      }
    });

    // Return the raw key ONLY ONCE. It is never stored in plaintext in the DB.
    return {
      id: apiKey.id,
      name: apiKey.name,
      tenantId: apiKey.tenantId,
      keyPrefix: apiKey.keyPrefix,
      rawApiKey: fullKey,
      createdAt: apiKey.createdAt
    };
  }
}
