import { ApiProperty } from '@nestjs/swagger';
import { IsString, IsNotEmpty } from 'class-validator';

export class CreateApiKeyDto {
  @ApiProperty({ example: 'tenant-id-uuid', description: 'The UUID of the tenant' })
  @IsString()
  @IsNotEmpty()
  tenantId!: string;

  @ApiProperty({ example: 'Production Key', description: 'Display name for the key' })
  @IsString()
  @IsNotEmpty()
  name!: string;
}
