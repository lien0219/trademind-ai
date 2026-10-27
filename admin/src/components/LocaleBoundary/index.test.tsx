import { act, render, screen, cleanup } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useAntdConfigSetter } from '@umijs/max';
import LocaleBoundary from '.';
import { setLanguage, tr } from '@/i18n';

afterEach(() => { cleanup(); setLanguage('zh-CN'); localStorage.clear(); });

it('switches the current view and component locale without a page reload', () => {
  setLanguage('zh-CN');
  const configure = vi.fn();
  vi.mocked(useAntdConfigSetter).mockReturnValue(configure);
  function Content() { return <span>{tr('登录')}</span>; }
  render(<LocaleBoundary><Content /></LocaleBoundary>);
  expect(screen.getByText('登录')).toBeVisible();
  act(() => setLanguage('en-US'));
  expect(screen.getByText('Log In')).toBeVisible();
  expect(configure).toHaveBeenLastCalledWith(expect.objectContaining({ locale: expect.objectContaining({ locale: 'en' }) }));
});
