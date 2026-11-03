import { expect, type Page, type Route } from '@playwright/test';
import { ok } from '../mocks/envelope';

type HttpMethod = 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export type WriteRecord = {
  operation: string;
  method: HttpMethod;
  url: string;
  path: string;
  query: Record<string, string>;
  headers: Record<string, string>;
  postDataJSON: unknown;
  order: number;
};

type AllowRule = {
  operation: string;
  method: HttpMethod;
  path: RegExp;
  response?: unknown | ((record: WriteRecord) => unknown | Promise<unknown>);
  status?: number;
};

export class NetworkWriteGuard {
  private readonly allowed: AllowRule[] = [];
  private readonly records: WriteRecord[] = [];
  private unexpected: WriteRecord[] = [];
  private order = 0;

  constructor(private readonly page: Page) {}

  async install() {
    await this.page.route('**/api/**', async (route) => this.handle(route));
  }

  allow(rule: AllowRule) {
    this.allowed.push(rule);
  }

  calls(operation: string) {
    return this.records.filter((record) => record.operation === operation);
  }

  allCalls() {
    return [...this.records];
  }

  unexpectedWrites() {
    return [...this.unexpected];
  }

  async expectNoUnexpectedWrites() {
    expect(this.unexpected, this.formatUnexpected()).toHaveLength(0);
  }

  async expectRequestCount(operation: string, count: number) {
    await expect.poll(() => this.calls(operation).length, { message: `${operation} request count` }).toBe(count);
  }

  private async handle(route: Route) {
    const request = route.request();
    const method = request.method().toUpperCase();
    if (!['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
      await route.fallback();
      return;
    }

    const url = new URL(request.url());
    let postDataJSON: unknown;
    try {
      postDataJSON = request.postDataJSON();
    } catch {
      postDataJSON = request.postData();
    }
    const record: WriteRecord = {
      operation: 'unexpected',
      method: method as HttpMethod,
      url: request.url(),
      path: url.pathname,
      query: Object.fromEntries(url.searchParams.entries()),
      headers: await request.allHeaders(),
      postDataJSON,
      order: ++this.order,
    };

    const matched = this.allowed.find((rule) => rule.method === method && rule.path.test(url.pathname));
    if (!matched) {
      this.unexpected.push(record);
      await route.fulfill({ status: 599, contentType: 'application/json', body: JSON.stringify({ code: 599, message: `unexpected ${method} ${url.pathname}`, data: null }) });
      return;
    }

    record.operation = matched.operation;
    this.records.push(record);
    const response = typeof matched.response === 'function' ? await matched.response(record) : matched.response ?? ok({});
    await route.fulfill({ status: matched.status ?? 200, contentType: 'application/json', body: JSON.stringify(response) });
  }

  private formatUnexpected() {
    return this.unexpected.map((record) => `${record.method} ${record.url} payload=${JSON.stringify(record.postDataJSON)}`).join('\n');
  }
}
