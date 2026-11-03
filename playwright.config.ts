/// <reference types="node" />

import { defineConfig, devices } from '@playwright/test';

const isCI = !!process.env.CI;
const defaultAdminE2EBaseURL = 'http://127.0.0.1:8001';

function resolveAdminE2EBaseURL(rawValue: string | undefined): string | undefined {
  const value = rawValue?.trim();
  if (!value) return undefined;

  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new Error('ADMIN_E2E_BASE_URL must be a valid absolute URL');
  }

  const isLoopbackHost = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname.toLowerCase());
  const isHttp = url.protocol === 'http:' || url.protocol === 'https:';
  const isOriginOnly = url.pathname === '/' && !url.search && !url.hash;

  if (!isHttp || !isLoopbackHost || !isOriginOnly || url.username || url.password) {
    throw new Error(
      'ADMIN_E2E_BASE_URL must be an http(s) loopback origin using localhost, 127.0.0.1, or [::1]',
    );
  }

  return url.origin;
}

const adminE2EBaseURL = resolveAdminE2EBaseURL(process.env.ADMIN_E2E_BASE_URL);

export default defineConfig({
  testDir: './admin/e2e/specs',
  outputDir: './test-results/admin-e2e',
  fullyParallel: false,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  workers: isCI ? 1 : undefined,
  timeout: 45_000,
  expect: { timeout: 8_000 },
  reporter: isCI
    ? [
        ['list'],
        ['html', { outputFolder: 'playwright-report/admin-e2e', open: 'never' }],
        ['junit', { outputFile: 'test-results/admin-e2e-junit.xml' }],
      ]
    : [['list'], ['html', { outputFolder: 'playwright-report/admin-e2e', open: 'never' }]],
  use: {
    baseURL: adminE2EBaseURL ?? defaultAdminE2EBaseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: adminE2EBaseURL
    ? undefined
    : {
        command: isCI
          ? 'pnpm build:admin && pnpm --filter @trademind/admin exec max preview --host 127.0.0.1 --port 8001'
          : 'pnpm dev:admin -- --host 127.0.0.1 --port 8001',
        url: defaultAdminE2EBaseURL,
        reuseExistingServer: !isCI,
        timeout: 120_000,
        env: {
          PORT: '8001',
          HOST: '127.0.0.1',
        },
      },
});
