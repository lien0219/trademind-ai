import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { BrowserManager } from '../browser/manager.js';
import { getHttpPort } from '../config/env.js';
import { listRegisteredSources } from '../providers/registry.js';
import { runCollectTask } from '../tasks/collect-task.js';

function json(res: ServerResponse, status: number, body: unknown): void {
  const buf = Buffer.from(JSON.stringify(body), 'utf8');
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': buf.length,
  });
  res.end(buf);
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
        const b = body as { source?: string; url?: string };
        const result = await runCollectTask(
          { source: b.source ?? '', url: b.url ?? '' },
          browser,
        );
        if (result.status === 'success') {
          json(res, 200, { ok: true, data: { product: result.product } });
        } else {
          json(res, 422, { ok: false, error: result.error });
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
    console.info(`[collector] listening on :${port} (POST /v1/collect, GET /health)`);
  });
  return server;
}
