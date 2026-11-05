import '@testing-library/jest-dom/vitest';
import React from 'react';
import { beforeEach, vi } from 'vitest';

const request = vi.fn();
const useAntdConfigSetter = vi.fn(() => vi.fn());
const history = {
  location: { pathname: '/', search: '' },
  push: vi.fn(),
  replace: vi.fn(),
};

vi.mock('@umijs/max', () => ({
  request,
  history,
  Link: ({ children, to, ...props }: { children?: React.ReactNode; to?: string }) =>
    React.createElement('a', { href: to, ...props }, children),
  useModel: vi.fn(() => ({})),
  useAccess: vi.fn(() => ({})),
  useAntdConfigSetter,
}));

const createMatchMedia = () => vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));

Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  writable: true,
  value: createMatchMedia(),
});

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

class IntersectionObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

Object.defineProperty(window, 'ResizeObserver', { writable: true, value: ResizeObserverMock });
Object.defineProperty(window, 'IntersectionObserver', { writable: true, value: IntersectionObserverMock });

beforeEach(() => {
  window.matchMedia = createMatchMedia();
  request.mockReset();
  useAntdConfigSetter.mockClear();
  useAntdConfigSetter.mockReturnValue(vi.fn());
  history.push.mockReset();
  history.replace.mockReset();
  history.location.pathname = '/';
  history.location.search = '';
});
