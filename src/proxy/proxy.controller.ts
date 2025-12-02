import { All, Controller, HttpStatus, Req, Res } from '@nestjs/common';
import type { Request, Response } from 'express';
import { MeterGateConfigService } from '../config/metergate-config.service';
import { StructuredLoggerService } from '../observability/structured-logger.service';
import { OtelService } from '../observability/otel.service';
import { PolicyEngineService } from '../policy/policy-engine.service';

const metergateInternalPrefixes = ['/health', '/docs', '/docs-json', '/v1/check'];

@Controller()
export class ProxyController {
  constructor(
    private readonly config: MeterGateConfigService,
    private readonly policy: PolicyEngineService,
    private readonly logger: StructuredLoggerService,
    private readonly otel: OtelService
  ) {}

  @All('{*proxyPath}')
  async proxy(@Req() request: Request, @Res() response: Response): Promise<void> {
    const requestId = request.requestId ?? 'missing-request-id';
    if (metergateInternalPrefixes.some((prefix) => request.path.startsWith(prefix))) {
      response.status(HttpStatus.NOT_FOUND).json({ message: 'Not found' });
      return;
    }

    const headerName = this.config.getApiKeyHeader();
    const apiKey = String(request.headers[headerName] ?? '');
    const decision = await this.policy.decide({
      method: request.method,
      path: request.path,
      apiKey: apiKey.length > 0 ? apiKey : undefined,
      requestId
    });

    response.setHeader('X-MeterGate-Decision', decision.reason);
    if (decision.limit !== undefined) {
      response.setHeader('X-RateLimit-Limit', String(decision.limit));
      response.setHeader('X-RateLimit-Remaining', String(decision.remaining ?? 0));
      response.setHeader('X-RateLimit-Reset', String(decision.resetSeconds ?? 0));
    }

    if (!decision.allowed) {
      response.status(decision.statusCode).json(decision);
      return;
    }

    await this.otel.span('metergate.proxy_forward', requestId, { path: request.path }, async () => {
      const upstream = new URL(request.originalUrl, this.config.get().proxy.upstreamUrl);
      const controller = new AbortController();
      const timer = setTimeout(() => {
        controller.abort();
      }, this.config.get().proxy.upstreamTimeoutMs);

      try {
        const upstreamResponse = await fetch(upstream, {
          method: request.method,
          headers: this.forwardHeaders(request, requestId),
          body: this.requestBody(request),
          signal: controller.signal
        });

        response.status(upstreamResponse.status);
        upstreamResponse.headers.forEach((value, key) => {
          if (!['set-cookie'].includes(key.toLowerCase())) {
            response.setHeader(key, value);
          }
        });
        response.send(Buffer.from(await upstreamResponse.arrayBuffer()));
      } catch (error) {
        this.logger.error('proxy forwarding failed', error instanceof Error ? error.stack : undefined, 'Proxy');
        response.status(502).json({ message: 'upstream unavailable', requestId });
      } finally {
        clearTimeout(timer);
      }
    });
  }

  private forwardHeaders(request: Request, requestId: string): Headers {
    const headers = new Headers();
    for (const [key, value] of Object.entries(request.headers)) {
      if (
        value === undefined ||
        ['connection', 'content-length', 'host', 'transfer-encoding'].includes(key.toLowerCase())
      ) {
        continue;
      }
      headers.set(key, Array.isArray(value) ? value.join(',') : value);
    }
    headers.set('x-request-id', requestId);
    return headers;
  }

  private requestBody(request: Request): string | undefined {
    if (['GET', 'HEAD'].includes(request.method.toUpperCase())) {
      return undefined;
    }
    return typeof request.body === 'string' ? request.body : JSON.stringify(request.body ?? {});
  }
}
