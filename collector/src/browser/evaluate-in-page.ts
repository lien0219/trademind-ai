import type { Page } from 'playwright';

/**
 * Run logic in the page context. Under tsx/esbuild `keepNames`, serialized callbacks
 * reference a Node-only `__name` helper — pass fn source as data and eval with a shim.
 */
export async function evaluateInPage<A, R>(page: Page, pageFunction: (arg: A) => R, arg: A): Promise<R> {
  const fnSource = pageFunction.toString();
  return page.evaluate(
    ({ innerArg, fnSource: src }) =>
      (0, eval)(
        `(function(__name, arg) { return (${src})(arg); })`,
      )(
        function (target: unknown, value: string) {
          Object.defineProperty(target as object, 'name', { value, configurable: true });
          return target;
        },
        innerArg,
      ),
    { innerArg: arg, fnSource },
  );
}

/** Zero-arg variant for simple DOM reads. */
export async function evaluateInPageVoid<R>(page: Page, pageFunction: () => R): Promise<R> {
  const fnSource = pageFunction.toString();
  return page.evaluate(
    ({ fnSource: src }) =>
      (0, eval)(`(function(__name) { return (${src})(); })`)(function (target: unknown, value: string) {
        Object.defineProperty(target as object, 'name', { value, configurable: true });
        return target;
      }),
    { fnSource },
  );
}
