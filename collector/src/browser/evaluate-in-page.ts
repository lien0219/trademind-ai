import type { Page } from 'playwright';

/**
 * esbuild/tsx keepNames may inject __name(...) into serialized evaluate callbacks.
 * Define a browser-global noop before any page.evaluate runs.
 */
export const PAGE_EVALUATE_POLYFILL = () => {
  const g = globalThis as { __name?: (target: unknown, value?: string) => unknown };
  if (typeof g.__name !== 'function') {
    g.__name = (target: unknown) => target;
  }
};

export async function ensurePageEvaluatePolyfill(page: Page): Promise<void> {
  await page.evaluate(PAGE_EVALUATE_POLYFILL);
}

/** Run serializable logic in the page via Playwright native evaluate (no toString/eval). */
export async function evaluateInPage<A, R>(
  page: Page,
  pageFunction: (arg: A) => R | Promise<R>,
  arg: A,
): Promise<R> {
  // Playwright serializes arg as Unboxed<A>; runtime shape matches our plain-object args.
  return page.evaluate(pageFunction as never, arg as never);
}

/** Zero-arg variant for simple DOM reads. */
export async function evaluateInPageVoid<R>(page: Page, pageFunction: () => R): Promise<R> {
  return page.evaluate(pageFunction);
}
