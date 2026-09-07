import { Controller, Get, Post, Delete, Param, Body, HttpCode, HttpStatus, ConflictException, NotFoundException } from '@nestjs/common';
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

  @Get()
  @ApiOperation({ summary: 'List all Tenants' })
  @ApiResponse({ status: 200, description: 'List of tenants' })
  async findAll() {
    return this.prisma.tenant.findMany({
      orderBy: { createdAt: 'desc' }
    });
  }

  @Get(':id')
  @ApiOperation({ summary: 'Get a Tenant by ID' })
  @ApiResponse({ status: 200, description: 'The tenant' })
  @ApiResponse({ status: 404, description: 'Tenant not found' })
  async findOne(@Param('id') id: string) {
    const tenant = await this.prisma.tenant.findUnique({
      where: { id }
    });
    if (!tenant) throw new NotFoundException('Tenant not found');
    return tenant;
  }

  @Delete(':id')
  @HttpCode(HttpStatus.NO_CONTENT)
  @ApiOperation({ summary: 'Delete a Tenant' })
  @ApiResponse({ status: 204, description: 'Tenant successfully deleted' })
  @ApiResponse({ status: 404, description: 'Tenant not found' })
  async remove(@Param('id') id: string) {
    const exists = await this.prisma.tenant.findUnique({ where: { id } });
    if (!exists) throw new NotFoundException('Tenant not found');

    await this.prisma.tenant.delete({
      where: { id }
    });
  }
}
