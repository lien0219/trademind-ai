import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { BrowserManager } from '../browser/manager.js';
import { getHttpPort } from '../config/env.js';
import { listRegisteredSources, listProviderPublicMetas } from '../providers/registry.js';
import { runCustomRuleTest } from '../providers/sourceCustom/index.js';
import { analyzeCustomPage } from '../providers/sourceCustom/analyze-page.js';
import type { CustomCollectOptions } from '../providers/sourceCustom/types.js';
import { runCollectTask } from '../tasks/collect-task.js';

function json(res: ServerResponse, status: number, body: unknown): void {
  const buf = Buffer.from(JSON.stringify(body), 'utf8');
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': buf.length,
  });
  res.end(buf);
}

function matchBrowserProfileRoute(
  method: string,
  url: string,
): { profileKey: string; action: 'open-login' | 'check' } | null {
  if (method !== 'POST') return null;
  const path = url.split('?')[0] ?? '';
  const m = path.match(/^\/v1\/browser-profiles\/([^/]+)\/(open-login|check)$/);
  if (!m) return null;
  return { profileKey: decodeURIComponent(m[1]), action: m[2] as 'open-login' | 'check' };
}

async function readJsonBody(req: IncomingMessage): Promise<unknown> {
  const raw = await new Promise<string>((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on('data', (c) => chunks.push(Buffer.from(c)));
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
  if (!raw.trim()) return {};
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    throw new Error('invalid_json');
  }
}

/**
 * HTTP 任务入口：POST /v1/collect
 * body: { "source": "1688", "url": "https://..." }
 */
export function createCollectorServer(browser: BrowserManager) {
  return createServer(async (req, res) => {
    try {
      if (req.method === 'GET' && req.url === '/health') {
        json(res, 200, {
          ok: true,
          service: 'trademind-collector',
          sources: listRegisteredSources(),
        });
        return;
      }

      if (req.method === 'GET' && req.url === '/v1/providers') {
        json(res, 200, { ok: true, data: listProviderPublicMetas() });
        return;
      }

      if (req.method === 'GET' && req.url === '/v1/providers/1688/auth-status') {
        const status = await browser.sessions.check1688AuthStatus();
        json(res, 200, { ok: true, data: status });
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/providers/1688/open-login-browser') {
        const result = await browser.sessions.openLoginBrowser('1688');
        json(res, 200, { ok: true, data: result });
        return;
      }

      const profileRoute = matchBrowserProfileRoute(req.method ?? '', req.url ?? '');
      if (profileRoute) {
        let body: unknown = {};
        try {
          body = await readJsonBody(req);
        } catch {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'body must be valid JSON' },
          });
          return;
        }
        const url = String((body as { url?: string }).url ?? '').trim();
        if (!url) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url is required' },
          });
          return;
        }
        try {
          if (profileRoute.action === 'open-login') {
            const data = await browser.customProfiles.openLoginBrowser(profileRoute.profileKey, url);
            json(res, 200, { ok: true, data });
            return;
          }
          const data = await browser.customProfiles.checkProfileAccess(profileRoute.profileKey, url);
          json(res, 200, { ok: true, data });
        } catch (e) {
          const message = e instanceof Error ? e.message : String(e);
          const code = message.startsWith('HEADED_BROWSER_REQUIRED')
            ? 'HEADED_BROWSER_REQUIRED'
            : message.startsWith('INVALID_PROFILE_KEY')
              ? 'INVALID_PROFILE_KEY'
              : 'INTERNAL';
          const status = code === 'HEADED_BROWSER_REQUIRED' ? 422 : code === 'INVALID_PROFILE_KEY' ? 400 : 500;
          json(res, status, { ok: false, error: { code, message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/custom/analyze-page') {
        let body: unknown;
        try {
          body = await readJsonBody(req);
        } catch {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'body must be valid JSON' },
          });
          return;
        }
        const b = body as {
          url?: string;
          profileKey?: string;
          useBrowserProfile?: boolean;
          maxCandidates?: number;
        };
        const url = b.url?.trim() ?? '';
        if (!url) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url is required' },
          });
          return;
        }
        try {
          const digest = await analyzeCustomPage(browser, url, {
            profileKey: b.profileKey,
            useBrowserProfile: b.useBrowserProfile,
            maxCandidates: b.maxCandidates,
          });
          json(res, 200, { ok: true, data: digest });
        } catch (e) {
          const message = e instanceof Error ? e.message : String(e);
          json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/collect/custom-rule-test') {
        let body: unknown;
        try {
          body = await readJsonBody(req);
        } catch {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'body must be valid JSON' },
          });
          return;
        }
        const b = body as { url?: string; options?: CustomCollectOptions };
        const url = b.url?.trim() ?? '';
        const opts = b.options;
        if (!url || !opts?.rule) {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'url and options.rule are required' },
          });
          return;
        }
        try {
          const result = await runCustomRuleTest(browser, url, opts);
          json(res, 200, {
            ok: true,
            data: {
              accessStatus: result.report.accessStatus,
              finalUrl: result.report.finalUrl,
              httpStatus: result.report.httpStatus,
              extractedFields: result.report.extractedFields,
              missingFields: result.report.missingFields,
              warnings: result.report.warnings,
              errorCode: result.report.errorCode,
              suggestion: result.report.suggestion,
              product: result.product ?? null,
            },
          });
        } catch (e) {
          const message = e instanceof Error ? e.message : String(e);
          json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
        }
        return;
      }

      if (req.method === 'POST' && req.url === '/v1/collect') {
        let body: unknown;
        try {
          body = await readJsonBody(req);
        } catch {
          json(res, 400, {
            ok: false,
            error: { code: 'INVALID_REQUEST', message: 'body must be valid JSON' },
          });
          return;
        }
        const b = body as { source?: string; url?: string; options?: Record<string, unknown> };
        const result = await runCollectTask(
          { source: b.source ?? '', url: b.url ?? '', options: b.options },
          browser,
        );
        if (result.status === 'success') {
          json(res, 200, { ok: true, data: { product: result.product } });
        } else {
          json(res, 422, {
            ok: false,
            error: result.error,
            data: result.access ? { accessReport: result.access } : undefined,
          });
        }
        return;
      }

      json(res, 404, {
        ok: false,
        error: { code: 'NOT_FOUND' as const, message: String(req.url ?? '') },
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      json(res, 500, { ok: false, error: { code: 'INTERNAL', message } });
    }
  });
}

export function listenCollectorHttp(browser: BrowserManager): ReturnType<typeof createServer> {
  const server = createCollectorServer(browser);
  const port = getHttpPort();
  server.listen(port, () => {
    console.info(
      `[collector] listening on :${port} (POST /v1/collect, POST /v1/custom/analyze-page, GET /v1/providers, GET /v1/providers/1688/auth-status, POST /v1/providers/1688/open-login-browser, GET /health)`,
    );
  });
  return server;
}
