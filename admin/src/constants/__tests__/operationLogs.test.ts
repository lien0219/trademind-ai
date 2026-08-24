import { describe, expect, it } from 'vitest';
import {
  operationLogActionLabel,
  operationLogResourceLabel,
} from '../operationLogs';

describe('operation log alert labels', () => {
  it('keeps system alert audit records user-facing', () => {
    expect(operationLogResourceLabel('alert_event')).toBe('系统告警');
    expect(operationLogActionLabel('alert.acknowledge')).toBe('确认系统告警');
    expect(operationLogActionLabel('alert.silence')).toBe('静默系统告警');
  });

  it('keeps purchase return audit records user-facing', () => {
    expect(operationLogResourceLabel('purchase_return')).toBe('采购退货单');
    expect(operationLogActionLabel('procurement.purchase_return.create')).toBe('创建采购退货单');
    expect(operationLogActionLabel('procurement.purchase_return.complete')).toBe('执行采购退货');
  });

  it('keeps sales return audit records user-facing', () => {
    expect(operationLogResourceLabel('sales_return')).toBe('销售售后单');
    expect(operationLogActionLabel('sales_return.create')).toBe(
      '创建销售售后单',
    );
    expect(operationLogActionLabel('sales_return.complete')).toBe(
      '完成销售售后单',
    );
    expect(
      operationLogActionLabel('sales_return.refund_execution.record_result'),
    ).toBe('登记退款执行结果');
  });
});
