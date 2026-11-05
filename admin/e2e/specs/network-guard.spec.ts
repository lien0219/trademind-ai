import { test, expect, type Page } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { NetworkWriteGuard } from '../utils/network-guard';

async function setupSyntheticApi(page: Page) {
  const bypassedRequests: string[] = [];
  await page.context().route('**/api/**', async (route) => {
    bypassedRequests.push(`${route.request().method()} ${new URL(route.request().url()).pathname}`);
    await route.fulfill({ json: ok({ source: 'synthetic-fallback' }) });
  });
  await page.route('**/__e2e-write-guard__', (route) =>
    route.fulfill({ contentType: 'text/html', body: '<html><body>Write guard regression</body></html>' }),
  );
  const guard = new NetworkWriteGuard(page);
  await guard.install();
  await page.goto('/__e2e-write-guard__');
  return { guard, bypassedRequests };
}

for (const path of [
  '/api/v1/e2e/write-guard',
  '/api/collector/providers/1688/open-login-browser',
  '/api/e2e/write-guard',
]) {
  test(`@smoke blocks undeclared API writes at ${path}`, async ({ page }) => {
    const { guard, bypassedRequests } = await setupSyntheticApi(page);
    const methods = ['POST', 'PUT', 'PATCH', 'DELETE'];
    const statuses = await page.evaluate(async ({ path, methods }) => {
      const statuses: number[] = [];
      for (const method of methods) {
        const response = await fetch(`${path}?source=e2e`, {
          method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ synthetic: true }),
        });
        statuses.push(response.status);
      }
      return statuses;
    }, { path, methods });

    expect(statuses).toEqual([599, 599, 599, 599]);
    expect(bypassedRequests).toEqual([]);
    expect(guard.allCalls()).toEqual([]);
    expect(guard.unexpectedWrites()).toEqual(methods.map((method, index) =>
      expect.objectContaining({
        operation: 'unexpected',
        method,
        path,
        query: { source: 'e2e' },
        postDataJSON: { synthetic: true },
        order: index + 1,
      }),
    ));
  });
}

test('@smoke mocks and records an explicitly allowed legacy Collector write', async ({ page }) => {
  const { guard, bypassedRequests } = await setupSyntheticApi(page);
  guard.allow({
    operation: 'e2e-open-login',
    method: 'POST',
    path: /^\/api\/collector\/providers\/1688\/open-login-browser$/,
    response: ok({ source: 'explicit-mock' }),
  });

  const result = await page.evaluate(async () => {
    const response = await fetch('/api/collector/providers/1688/open-login-browser?source=e2e', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ synthetic: true }),
    });
    return { status: response.status, body: await response.json() };
  });

  expect(result).toEqual({ status: 200, body: ok({ source: 'explicit-mock' }) });
  expect(bypassedRequests).toEqual([]);
  await guard.expectRequestCount('e2e-open-login', 1);
  expect(guard.calls('e2e-open-login')[0]).toEqual(expect.objectContaining({
    method: 'POST',
    path: '/api/collector/providers/1688/open-login-browser',
    query: { source: 'e2e' },
    postDataJSON: { synthetic: true },
    order: 1,
  }));
  await guard.expectNoUnexpectedWrites();
});

test('@smoke lets API reads reach the synthetic read mock', async ({ page }) => {
  const { guard, bypassedRequests } = await setupSyntheticApi(page);
  const statuses = await page.evaluate(async () => {
    const statuses: number[] = [];
    for (const method of ['GET', 'HEAD', 'OPTIONS']) {
      const response = await fetch('/api/collector/providers/1688/auth-status', { method });
      statuses.push(response.status);
    }
    return statuses;
  });

  expect(statuses).toEqual([200, 200, 200]);
  expect(bypassedRequests).toHaveLength(3);
  expect(guard.allCalls()).toEqual([]);
  await guard.expectNoUnexpectedWrites();
});
