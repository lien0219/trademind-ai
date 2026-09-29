import type { PurchaseOrderStatus } from '@/services/procurement';

export const PURCHASE_ORDER_STATUS: Record<string, { text: string; color: string }> = {
  draft: { text: '草稿', color: 'default' },
  pending_approval: { text: '待审批', color: 'processing' },
  approved: { text: '待收货', color: 'blue' },
  partially_received: { text: '部分收货', color: 'warning' },
  received: { text: '已收货', color: 'success' },
  closed: { text: '已关闭', color: 'default' },
  cancelled: { text: '已取消', color: 'default' },
};

export function parseMajorAmountToMinor(value: string | number | undefined): number {
  const raw = String(value ?? '').trim();
  if (!/^\d{1,13}(?:\.\d{1,2})?$/.test(raw)) {
    throw new Error('金额最多保留两位小数');
  }
  const [whole, fraction = ''] = raw.split('.');
  const minor = Number(whole) * 100 + Number(fraction.padEnd(2, '0'));
  if (!Number.isSafeInteger(minor)) {
    throw new Error('金额超出可处理范围');
  }
  return minor;
}

export function formatMinorAmount(value: number, currency = 'CNY') {
  if (!Number.isSafeInteger(value)) return '—';
  const negative = value < 0;
  const absolute = Math.abs(value);
  const whole = Math.floor(absolute / 100);
  const fraction = String(absolute % 100).padStart(2, '0');
  return `${currency.toUpperCase()} ${negative ? '-' : ''}${whole}.${fraction}`;
}

export function remainingQuantity(quantity: number, receivedQuantity: number) {
  return Math.max(0, quantity - receivedQuantity);
}

export function canReceivePurchaseOrder(status: PurchaseOrderStatus | string) {
  return status === 'approved' || status === 'partially_received';
}
