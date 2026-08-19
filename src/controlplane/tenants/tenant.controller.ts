import { Controller, Post, Body, HttpCode, HttpStatus, ConflictException } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse } from '@nestjs/swagger';
import { PrismaService } from './prisma.service';
import { CreateTenantDto } from './dto/create-tenant.dto';

@ApiTags('Tenants')
@Controller('tenants')
export class TenantController {
  constructor(private readonly prisma: PrismaService) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  @ApiOperation({ summary: 'Provision a new Tenant' })
  @ApiResponse({ status: 201, description: 'Tenant successfully created' })
  @ApiResponse({ status: 409, description: 'Tenant slug already exists' })
  async create(@Body() createTenantDto: CreateTenantDto) {
    const exists = await this.prisma.tenant.findUnique({
      where: { slug: createTenantDto.slug }
    });

    if (exists) {
      throw new ConflictException('Tenant slug already exists');
    }

    const tenant = await this.prisma.tenant.create({
      data: {
        slug: createTenantDto.slug,
        name: createTenantDto.name,
        planCode: createTenantDto.planCode
      }
    });

    return tenant;
  }
}
