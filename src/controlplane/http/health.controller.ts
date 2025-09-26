import { Controller, Get } from '@nestjs/common';
import { ApiOkResponse, ApiTags } from '@nestjs/swagger';

@ApiTags('health')
@Controller()
export class HealthController {
  @Get('/health')
  @ApiOkResponse({ description: 'MeterGate health status' })
  health(): { status: 'ok'; service: string } {
    return { status: 'ok', service: 'metergate' };
  }
}

