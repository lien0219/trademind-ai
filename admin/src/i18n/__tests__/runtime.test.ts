import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { supportedLanguages } from '../dictionaries';
import { applyLanguage, formatLocalizedCurrency, formatLocalizedDate, formatLocalizedNumber, LOCALE_STORAGE_KEY, readLanguage, setLanguage, setRegionalPreferences, subscribeLanguage, trFormat } from '..';
import { localizeApiError } from '../apiErrors';

beforeEach(() => {
  localStorage.clear();
  setLanguage('zh-CN');
  setRegionalPreferences({ dateFormat: 'YYYY-MM-DD', timeZone: 'UTC' });
});
afterEach(() => { vi.restoreAllMocks(); setLanguage('zh-CN'); localStorage.clear(); });

describe('locale runtime', () => {
  it.each(supportedLanguages)('persists %s and publishes it to the current view', language => {
    const receive = vi.fn();
    const dispose = subscribeLanguage(receive);
    setLanguage(language);
    expect(readLanguage()).toBe(language);
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe(language);
    expect(document.documentElement.lang).toBe(language);
    expect(receive).toHaveBeenLastCalledWith(language);
    dispose();
    setLanguage('zh-CN');
    expect(receive).toHaveBeenCalledTimes(1);
  });

  it('keeps a valid active language when storage rejects writes', () => {
    const receive = vi.fn();
    const dispose = subscribeLanguage(receive);
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage denied'); });
    setLanguage('ja-JP');
    expect(readLanguage()).toBe('ja-JP');
    expect(receive).toHaveBeenCalledWith('ja-JP');
    dispose();
  });

  it('ignores unrelated storage updates and handles a language change in another tab', () => {
    const receive = vi.fn();
    const dispose = subscribeLanguage(receive);
    window.dispatchEvent(new StorageEvent('storage', { key: 'unrelated' }));
    expect(receive).not.toHaveBeenCalled();
    localStorage.setItem(LOCALE_STORAGE_KEY, 'ko-KR');
    window.dispatchEvent(new StorageEvent('storage', { key: LOCALE_STORAGE_KEY, newValue: 'ko-KR' }));
    expect(receive).toHaveBeenCalledWith('ko-KR');
    dispose();
  });

  it('rejects unknown languages received at runtime', () => {
    setLanguage('invalid' as Parameters<typeof setLanguage>[0]);
    expect(readLanguage()).toBe('zh-CN');
    applyLanguage('en-US');
    expect(document.documentElement.lang).toBe('en-US');
  });
});

describe('regional display', () => {
  it.each(supportedLanguages)('uses Gregorian dates and the preferred format for %s', language => {
    setRegionalPreferences({ dateFormat: 'DD/MM/YYYY', timeZone: 'Asia/Tokyo' });
    expect(formatLocalizedDate('2026-10-27T17:08:09Z', language)).toBe('28/10/2026 02:08:09');
  });

  it('honors an explicit timezone and handles midnight without a 24th hour', () => {
    setRegionalPreferences({ dateFormat: 'MM/DD/YYYY', timeZone: 'Asia/Tokyo' });
    expect(formatLocalizedDate('2026-10-27T00:00:00Z', 'en-US', 'UTC')).toBe('10/27/2026 00:00:00');
    expect(formatLocalizedDate('invalid')).toBe('invalid');
  });

  it('falls back safely for invalid stored date formats and timezones', () => {
    setRegionalPreferences({ dateFormat: 'not-a-format', timeZone: 'not-a-zone' });
    expect(() => formatLocalizedDate('2026-10-27T00:00:00Z')).not.toThrow();
  });

  it('formats numbers and original currencies without conversion', () => {
    expect(formatLocalizedNumber(1234.5, undefined, 'en-US')).toBe('1,234.5');
    expect(formatLocalizedCurrency(1234.5, 'USD', 'en-US')).toBe('$1,234.50');
    expect(formatLocalizedCurrency(1234, 'JPY', 'ja-JP')).toContain('1,234');
  });
});

describe('plain-text message interpolation', () => {
  it.each(supportedLanguages)('preserves business values exactly in %s', language => {
    const name = '<script>{action}</script> 设置 $&';
    const result = trFormat('删除企业：{name}', { name }, language);
    expect(result).toContain(name);
    expect(result).not.toContain('undefined');
  });

  it('does not read inherited replacements or recursively substitute values', () => {
    expect(trFormat('{constructor} {value} {missing}', { value: '{missing}' }, 'en-US'))
      .toBe('{constructor} {missing} {missing}');
    expect(trFormat('共 {count} 家企业', { count: 0 }, 'en-US')).toBe('Organizations: 0');
  });
});

describe('public API errors', () => {
  it.each(supportedLanguages.filter(language => language !== 'zh-CN'))('localizes unknown server errors in %s', language => {
    setLanguage(language);
    const message = localizeApiError(40901, 'settings version conflict; reload');
    expect(message).toBeTruthy();
    expect(message).not.toContain('settings version conflict');
    expect(message).not.toBe('数据已被更新，请刷新后重试');
  });

  it('retains default Chinese business messages', () => {
    expect(localizeApiError(40001, '商品不存在')).toBe('商品不存在');
  });
});
