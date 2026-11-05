import { act, fireEvent, render, screen, cleanup } from '@testing-library/react';
import { useEffect, useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import { useAntdConfigSetter } from '@umijs/max';
import LocaleBoundary from '.';
import { setLanguage, tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';

afterEach(() => { cleanup(); setLanguage('zh-CN'); localStorage.clear(); });

it('switches the current view and component locale without a page reload', () => {
  setLanguage('zh-CN');
  const configure = vi.fn();
  vi.mocked(useAntdConfigSetter).mockReturnValue(configure);
  function Content() { useLanguage(); return <span>{tr('登录')}</span>; }
  render(<LocaleBoundary><Content /></LocaleBoundary>);
  expect(screen.getByText('登录')).toBeVisible();
  act(() => setLanguage('en-US'));
  expect(screen.getByText('Log In')).toBeVisible();
  expect(configure).toHaveBeenLastCalledWith(expect.objectContaining({ locale: expect.objectContaining({ locale: 'en' }) }));
});

it('preserves unsaved fields, component state and mount effects on a language change', () => {
  const mounted = vi.fn();
  const unmounted = vi.fn();
  function FormContent() {
    useLanguage();
    const [value, setValue] = useState('');
    useEffect(() => { mounted(); return unmounted; }, []);
    return <label>{tr('登录')}<input value={value} onChange={event => setValue(event.target.value)} /></label>;
  }
  render(<LocaleBoundary><FormContent /></LocaleBoundary>);
  fireEvent.change(screen.getByRole('textbox'), { target: { value: 'unsaved business data' } });
  const field = screen.getByRole('textbox');
  act(() => setLanguage('ja-JP'));
  expect(screen.getByText(tr('登录'))).toBeVisible();
  expect(screen.getByRole('textbox')).toBe(field);
  expect(field).toHaveValue('unsaved business data');
  expect(mounted).toHaveBeenCalledTimes(1);
  expect(unmounted).not.toHaveBeenCalled();
});

it('does not write the locale again when the Umi provider replaces its setter', () => {
  const configure = vi.fn();
  vi.mocked(useAntdConfigSetter).mockReturnValue(configure);
  const { rerender } = render(<LocaleBoundary><input defaultValue="draft" /></LocaleBoundary>);
  const field = screen.getByRole('textbox');
  expect(configure).toHaveBeenCalledTimes(1);

  // Umi creates a new setter after each config state update.
  const replacement = vi.fn();
  vi.mocked(useAntdConfigSetter).mockReturnValue(replacement);
  rerender(<LocaleBoundary><input defaultValue="draft" /></LocaleBoundary>);
  expect(replacement).not.toHaveBeenCalled();
  expect(screen.getByRole('textbox')).toBe(field);

  act(() => setLanguage('en-US'));
  expect(replacement).toHaveBeenCalledTimes(1);
  expect(replacement).toHaveBeenLastCalledWith(expect.objectContaining({ locale: expect.objectContaining({ locale: 'en' }) }));
  expect(screen.getByRole('textbox')).toBe(field);
});
