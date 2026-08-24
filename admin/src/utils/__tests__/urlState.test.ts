import { history } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { appendSourceToUrl, parsePositiveInt, parseProductionCreatePrefill, readQueryState, writeQueryState } from '../urlState';

const historyMock = vi.mocked(history);

describe('urlState helpers', () => {
  it('parses positive integers and falls back for invalid values', () => {
    expect(parsePositiveInt('3.8')).toBe(3);
    expect(parsePositiveInt('0', 7)).toBe(7);
    expect(parsePositiveInt('abc', 2)).toBe(2);
  });

  it('reads only requested query keys', () => {
    expect(readQueryState('?page=2&tab=publish&empty=', ['page', 'tab', 'empty'] as const)).toEqual({
      page: '2',
      tab: 'publish',
      empty: undefined,
    });
  });

  it('appends default navigation source without overwriting existing source', () => {
    expect(appendSourceToUrl('/products/p1')).toBe('/products/p1?source=dashboard');
    expect(appendSourceToUrl('/products/p1?source=taskcenter', 'manual')).toBe('/products/p1?source=taskcenter');
  });

  it('writes allowlisted inventory filters and drops unknown query keys', () => {
    historyMock.location.pathname = '/products';
    historyMock.location.search = '?page=1&keyword=old';

    writeQueryState({ page: 2, keyword: '', warehouseId: 'warehouse-main', dangerous: 'x' }, { replace: true });

    expect(historyMock.replace).toHaveBeenCalledWith('/products?page=2&warehouseId=warehouse-main');
    expect(historyMock.push).not.toHaveBeenCalled();
  });

  it('preserves operation-task return paths and cursor history', () => {
    historyMock.location.pathname = '/ops/task-center/operation-tasks/task-1';
    historyMock.location.search = '?tab=events';

    writeQueryState({
      from: '/ops/task-center/operation-tasks?status=execution_failed',
      cursorHistory: '["","cursor-1"]',
    }, { replace: true });

    const written = new URL(historyMock.replace.mock.calls.at(-1)?.[0] as string, 'https://admin.example');
    expect(written.searchParams.get('from')).toBe('/ops/task-center/operation-tasks?status=execution_failed');
    expect(written.searchParams.get('cursorHistory')).toBe('["","cursor-1"]');
  });

  it('accepts safe production create prefill values and rejects unsafe links', () => {
    expect(parseProductionCreatePrefill({
      create: 'production', productId: 'product-123', shopId: 'shop_456',
    })).toEqual({ productId: 'product-123', shopId: 'shop_456' });
    expect(parseProductionCreatePrefill({ create: 'local', productId: 'product-123' })).toBeUndefined();
    expect(parseProductionCreatePrefill({ create: 'production', productId: '../product' })).toBeUndefined();
    expect(parseProductionCreatePrefill({ create: 'production', productId: 'p'.repeat(65) })).toBeUndefined();
  });
});
