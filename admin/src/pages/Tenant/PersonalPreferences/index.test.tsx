import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { message } from 'antd';
import { getJSON, putJSON } from '@/services/request';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { readLanguage, setLanguage } from '@/i18n';
import PersonalPreferencesPage from '.';

vi.mock('@/services/request', () => ({ getJSON: vi.fn(), putJSON: vi.fn() }));
vi.mock('@/components/ui', () => ({ TmPageContainer: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));

const initial = { values: { language: 'zh-CN', theme: 'light', dateFormat: 'YYYY-MM-DD' }, version: 1, inherited: false };
beforeEach(() => {
  localStorage.clear();
  setLanguage('zh-CN');
  vi.mocked(getJSON).mockReset();
  vi.mocked(putJSON).mockReset();
  vi.mocked(getJSON).mockResolvedValue(initial);
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false, media: query, onchange: null,
      addListener: () => {}, removeListener: () => {},
      addEventListener: () => {}, removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
});
afterEach(() => { cleanup(); setLanguage('zh-CN'); localStorage.clear(); });

it('allows only one preference save while validation and the request are pending', async () => {
  let finish!: (value: typeof initial) => void;
  vi.mocked(putJSON).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  render(<PersonalPreferencesPage />);
  const save = await screen.findByRole('button', { name: '保存个人偏好' });
  fireEvent.click(save);
  fireEvent.click(save);
  await waitFor(() => expect(putJSON).toHaveBeenCalledTimes(1));
  expect(putJSON).toHaveBeenCalledWith('/api/v1/tenant/preferences', { values: initial.values, expectedVersion: 1 });
  await act(async () => { finish({ ...initial, version: 2 }); });
});

it('does not apply a save response from a previous authenticated workspace', async () => {
  let finish!: (value: typeof initial) => void;
  vi.mocked(putJSON).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const success = vi.spyOn(message, 'success');
  localStorage.setItem(AUTH_TOKEN_KEY, 'e2e-first-context');
  render(<PersonalPreferencesPage />);
  fireEvent.click(await screen.findByRole('button', { name: '保存个人偏好' }));
  await waitFor(() => expect(putJSON).toHaveBeenCalledTimes(1));
  localStorage.setItem(AUTH_TOKEN_KEY, 'e2e-second-context');
  await act(async () => { finish({ ...initial, values: { ...initial.values, language: 'ja-JP' }, version: 2 }); });
  expect(readLanguage()).toBe('zh-CN');
  expect(success).not.toHaveBeenCalled();
});
