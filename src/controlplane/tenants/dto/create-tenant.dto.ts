import { ApiProperty } from '@nestjs/swagger';
import { IsString, IsNotEmpty } from 'class-validator';

export class CreateTenantDto {
  @ApiProperty({ example: 'acme-corp', description: 'Unique slug for the tenant' })
  @IsString()
  @IsNotEmpty()
  slug!: string;

  @ApiProperty({ example: 'Acme Corporation', description: 'Display name' })
  @IsString()
  @IsNotEmpty()
  name!: string;

  @ApiProperty({ example: 'starter', description: 'The billing plan code for the tenant (must match config)' })
  @IsString()
  @IsNotEmpty()
  planCode!: string;
}
