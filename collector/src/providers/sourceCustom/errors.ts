import type { CustomAccessReport } from '../../types/access-status.js';

export class CustomCollectError extends Error {
  readonly code: string;
  readonly report: CustomAccessReport;

  constructor(code: string, report: CustomAccessReport, message?: string) {
    super(message ?? report.suggestion ?? code);
    this.name = 'CustomCollectError';
    this.code = code;
    this.report = report;
  }
}

export function throwCustomError(code: string, report: CustomAccessReport): never {
  throw new CustomCollectError(code, report);
}
