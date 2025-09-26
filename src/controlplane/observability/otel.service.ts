import { appendFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { Injectable } from '@nestjs/common';

export type SpanRecord = {
  name: string;
  requestId: string;
  attributes?: Record<string, unknown>;
  startTime: string;
  endTime: string;
  durationMs: number;
  traceId: string;
  spanId: string;
};

@Injectable()
export class OtelService {
  private readonly exporters = new Set((process.env.OTEL_EXPORTER ?? 'stdout').split(','));
  private readonly filePath = process.env.OTEL_FILE_PATH ?? 'otel-output/metergate.ndjson';

  async span<T>(
    name: string,
    requestId: string,
    attributes: Record<string, unknown>,
    operation: () => Promise<T>
  ): Promise<T> {
    const started = performance.now();
    const startTime = new Date().toISOString();
    try {
      return await operation();
    } finally {
      const durationMs = Number((performance.now() - started).toFixed(3));
      await this.emit({
        name,
        requestId,
        attributes,
        startTime,
        endTime: new Date().toISOString(),
        durationMs,
        traceId: requestId.replace(/[^a-f0-9]/gi, '').padEnd(32, '0').slice(0, 32),
        spanId: randomUUID().replaceAll('-', '').slice(0, 16)
      });
    }
  }

  private async emit(record: SpanRecord): Promise<void> {
    const line = `${JSON.stringify({ type: 'otel.span', ...record })}\n`;
    if (this.exporters.has('stdout')) {
      process.stdout.write(line);
    }
    if (this.exporters.has('file')) {
      await appendFile(this.filePath, line).catch(() => undefined);
    }
  }
}

