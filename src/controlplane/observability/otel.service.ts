import { Injectable, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { StructuredLoggerService } from './structured-logger.service';
import { trace, Tracer, SpanStatusCode, metrics, Meter, Histogram } from '@opentelemetry/api';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { ConsoleSpanExporter } from '@opentelemetry/sdk-trace-node';
import { PrometheusExporter } from '@opentelemetry/exporter-prometheus';

/**
 * OpenTelemetry instrumentation service.
 * Wraps async operations with real OpenTelemetry spans and Prometheus metrics.
 */
@Injectable()
export class OtelService implements OnModuleInit, OnModuleDestroy {
  private tracer: Tracer;
  private meter: Meter;
  private latencyHistogram: Histogram;
  private sdk: NodeSDK;

  constructor(private readonly logger: StructuredLoggerService) {
    this.sdk = new NodeSDK({
      traceExporter: new ConsoleSpanExporter(),
      metricReader: new PrometheusExporter({ port: 9464 }), // Exposes /metrics on port 9464
    });
    
    this.tracer = trace.getTracer('metergate-controlplane');
    this.meter = metrics.getMeter('metergate-controlplane');
    
    this.latencyHistogram = this.meter.createHistogram('metergate_controlplane_operation_duration_ms', {
      description: 'Duration of control plane operations in milliseconds',
      unit: 'ms',
    });
  }

  onModuleInit() {
    this.sdk.start();
    this.logger.log('OpenTelemetry SDK started. Prometheus metrics available on :9464/metrics', 'OtelService');
  }

  onModuleDestroy() {
    this.sdk.shutdown();
  }

  async span<T>(
    name: string,
    requestId: string,
    attributes: Record<string, unknown>,
    operation: () => Promise<T>
  ): Promise<T> {
    const start = performance.now();
    return new Promise((resolve, reject) => {
      this.tracer.startActiveSpan(name, { attributes: { requestId, ...attributes } }, async (span) => {
        let hasError = false;
        try {
          const result = await operation();
          span.setStatus({ code: SpanStatusCode.OK });
          resolve(result);
        } catch (error) {
          hasError = true;
          span.setStatus({
            code: SpanStatusCode.ERROR,
            message: error instanceof Error ? error.message : String(error),
          });
          if (error instanceof Error) {
            span.recordException(error);
          }
          reject(error);
        } finally {
          span.end();
          const duration = performance.now() - start;
          this.latencyHistogram.record(duration, { operation: name, error: String(hasError) });
        }
      });
    });
  }
}
