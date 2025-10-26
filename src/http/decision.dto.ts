import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { IsOptional, IsString } from 'class-validator';

export class CheckRequestDto {
  @ApiProperty({ example: 'POST' })
  @IsString()
  method!: string;

  @ApiProperty({ example: '/graphql' })
  @IsString()
  path!: string;

  @ApiPropertyOptional({ example: 'mg_demo_local_plaintext_key' })
  @IsOptional()
  @IsString()
  apiKey?: string;
}

export class CheckResponseDto {
  @ApiProperty()
  allowed!: boolean;

  @ApiProperty({ example: 200 })
  statusCode!: number;

  @ApiProperty({ example: 'ALLOWED' })
  reason!: string;

  @ApiPropertyOptional()
  tenantId?: string;

  @ApiPropertyOptional()
  tenantSlug?: string;

  @ApiPropertyOptional()
  routeId?: string;

  @ApiPropertyOptional()
  planCode?: string;

  @ApiPropertyOptional()
  limit?: number;

  @ApiPropertyOptional()
  remaining?: number;

  @ApiPropertyOptional()
  resetSeconds?: number;
}

