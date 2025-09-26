import { Injectable, LoggerService } from '@nestjs/common';

type LogRecord = {
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  timestamp: string;
  requestId?: string;
  [key: string]: unknown;
};

@Injectable()
export class StructuredLoggerService implements LoggerService {
  private readonly level = process.env.LOG_LEVEL ?? 'info';

  log(message: string, context?: string): void {
    this.write({ level: 'info', message, timestamp: new Date().toISOString(), context });
  }

  error(message: string, trace?: string, context?: string): void {
    this.write({ level: 'error', message, timestamp: new Date().toISOString(), trace, context });
  }

  warn(message: string, context?: string): void {
    this.write({ level: 'warn', message, timestamp: new Date().toISOString(), context });
  }

  debug(message: string, context?: string): void {
    if (this.level === 'debug') {
      this.write({ level: 'debug', message, timestamp: new Date().toISOString(), context });
    }
  }

  write(record: LogRecord): void {
    process.stdout.write(`${JSON.stringify(record)}\n`);
  }
}

