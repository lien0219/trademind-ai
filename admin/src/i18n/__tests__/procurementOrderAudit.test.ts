import { describe, expect, it } from 'vitest';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

describe('procurement and order UI copy audit', () => {
  it('has no untranslated user-facing Chinese candidates in either module', () => {
    const result = JSON.parse(execFileSync(
      process.execPath,
      ['scripts/i18n-audit.mjs', '--details'],
      { encoding: 'utf8' },
    )) as { candidates: Array<{ file: string; line: number; sample: string }> };
    const candidates = result.candidates.filter(({ file }) =>
      /^pages\/(?:Orders|Procurement|SalesReturns)\//.test(file),
    );

    expect(candidates).toEqual([]);
  });

  it('defers module metadata translation until the component renders', () => {
    const fulfillmentDocuments = readFileSync(
      'src/pages/Orders/FulfillmentDocuments/index.tsx',
      'utf8',
    );
    const warehouseAllocations = readFileSync(
      'src/pages/Orders/WarehouseAllocations/index.tsx',
      'utf8',
    );

    const documentTypes = fulfillmentDocuments.match(
      /const DOCUMENT_TYPES[\s\S]*?\];/,
    )?.[0] ?? '';
    const statusMeta = warehouseAllocations.match(
      /const STATUS_META[\s\S]*?} as const;/,
    )?.[0] ?? '';

    expect(documentTypes).not.toMatch(/\btr\(/);
    expect(statusMeta).not.toMatch(/\btr\(/);
  });

  it('keeps order and refund status labels current after a locale change', () => {
    const refundExecutions = readFileSync(
      'src/pages/SalesReturns/RefundExecutions.tsx',
      'utf8',
    );
    const orderList = readFileSync('src/pages/Orders/index.tsx', 'utf8');
    const fulfillmentReconciliation = readFileSync(
      'src/pages/Orders/FulfillmentReconciliation/index.tsx',
      'utf8',
    );
    const refundStatus = refundExecutions.match(
      /export const REFUND_EXECUTION_STATUS[\s\S]*?\n};/,
    )?.[0] ?? '';

    expect(refundStatus).not.toMatch(/text:\s*tr\(/);
    expect(refundStatus).toMatch(/get text\(\)\s*{\s*return tr\(/);
    expect(refundExecutions).toMatch(/\buseLanguage\(\)/);
    expect(orderList).not.toMatch(/^const (ORDER_STATUS_OPTS|PAY_OPTS|FULL_OPTS|SHIP_OPTS)\b/m);
    expect(orderList).not.toMatch(/setWarehouseOptions/);
    expect(orderList).toMatch(/const \[warehouses, setWarehouses\]/);
    expect(fulfillmentReconciliation).not.toMatch(
      /^const (RECONCILIATION_OPTIONS|ORDER_STATUS_OPTIONS|FULFILLMENT_STATUS_OPTIONS)\b/m,
    );
  });
});
