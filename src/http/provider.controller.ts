import { Body, Controller, Post, Req } from '@nestjs/common';
import { ApiOkResponse, ApiTags } from '@nestjs/swagger';
import type { Request } from 'express';
import { PolicyEngineService } from '../policy/policy-engine.service';
import type { PolicyDecision } from '../policy/policy.types';
import { CheckRequestDto, CheckResponseDto } from './decision.dto';

@ApiTags('provider')
@Controller('/v1')
export class ProviderController {
  constructor(private readonly policy: PolicyEngineService) {}

  @Post('/check')
  @ApiOkResponse({ type: CheckResponseDto })
  async check(@Body() body: CheckRequestDto, @Req() request: Request): Promise<PolicyDecision> {
    return this.policy.decide({
      method: body.method,
      path: body.path,
      apiKey: body.apiKey,
      requestId: request.requestId ?? 'missing-request-id'
    });
  }
}

