#!/usr/bin/env node
/**
 * 扫描用户可见路径中的常见英文/技术混排。
 * 用法: node scripts/check-ui-copy.mjs [--strict]
 */
import { readFileSync, readdirSync, statSync } from 'fs';
import { join, relative } from 'path';

const ROOT = join(import.meta.dirname, '..');
const STRICT = process.argv.includes('--strict');

/** @type {{ pattern: RegExp; hint: string; allow?: RegExp }[]} */
const RULES = [
  {
    pattern: /\bruntime\b/i,
    hint: '→ 运行时',
    allow: /platform-runtime|PlatformRuntime|runtime-status|runtimeBlocked|useState|setRuntime|runtimeStatusTag|runtime\?|PLATFORM_PROVIDER_STATUS|RELEASE_GATE|out\.Runtime|\.Runtime\s*=|\/\*\*|\/\/|\`json:/i,
  },
  {
    pattern: /\bWorker\b/,
    hint: '→ 后台任务进程',
    allow: /WorkerMonitor|getWorkersMonitor|workerType|workerId|WORKER_|\/\*\*|\/\//,
  },
  {
    pattern: /\bStorage\b/,
    hint: '→ 存储',
    allow: /Settings\/Storage|testStorage|storageKey|StoragePublic|storage\.|Preflight\.Storage|healthStorage|HealthSection|json:|`storage|\.Storage|\*storagepublic|Storage\s+\*|\/\*\*|\/\//,
  },
  {
    pattern: /\bProvider\b/,
    hint: '→ 接入方式/服务',
    allow: /ProviderMeta|queryPlatform|fetchImage|useImage|CollectProvider|PlatformProvider|ImageProvider|AIProvider|provider:|providerLabel|providerRow|loadProviders|\/\*\*|\/\//,
  },
  { pattern: /\bStale\b/, hint: '→ 停滞', allow: /stale24h|StaleTasks|recoveryStatus.*stale|\/\*\*|\/\// },
  {
    pattern: /\bRelease Candidate\b/,
    hint: '→ 发布候选',
    allow: /RELEASE_GATE|RELEASE_GATE_CONCLUSION|'Release Candidate':|\/\*\*|\/\//,
  },
  { pattern: /\bblocked_by_/i, hint: '→ 中文说明' },
  {
    pattern: /\bneed_check\b/,
    hint: '→ 需要检查',
    allow: /need_check:\s*['"`]需要检查|need_check:\s*['"`]warning|need_check:\s*\{\s*text:\s*['"`]需要检查|authStatus\s*===|\/\*\*|\/\//,
  },
  { pattern: /未接入 runtime/i, hint: '→ 未接入运行时' },
  { pattern: /待同步 Storage|同步 Storage|Storage 公网|Storage 已|Storage 正常/i, hint: '→ 存储' },
  {
    pattern: /message:\s*['"`][^'"`]*\bAPI\b[^'"`]*['"`]/,
    hint: '→ 接口',
    allow: /SP-API|OpenAI|接口密钥|接口地址|\/\*\*|\/\//,
  },
  {
    pattern: /(?:title|label|description|message|subTitle|placeholder|extra):\s*['"`][^'"`]*\bToken\b/,
    hint: '→ 访问令牌',
    allow: /token 或|\/\*\*|\/\//,
  },
  {
    pattern: /(?:title|label|message|placeholder|extra):\s*['"`][^'"`]*\bSKU\b/,
    hint: '→ 规格',
    allow: /SKU_ID|skuCode|skuName|SkuMatch|platformSku|productSku|\/\*\*|\/\//,
  },
  { pattern: /\bMock\b/, hint: '→ 模拟', allow: /mock_shop|mockShop|platform.*mock|\/\*\*|\/\// },
  {
    pattern: /Webhook/,
    hint: '→ 回调通知',
    allow: /webhook_|opsWebhookSecret|SECURITY_FIELD|feishu_webhook|wecom_webhook|\/\*\*|\/\//,
  },
];

const SCAN_DIRS = [
  join(ROOT, 'admin/src/pages'),
  join(ROOT, 'admin/src/constants'),
  join(ROOT, 'admin/src/components'),
  join(ROOT, 'backend/internal/modules/douyinpreflight'),
  join(ROOT, 'backend/internal/modules/douyinruntime'),
  join(ROOT, 'backend/internal/modules/taskcenter/failureclassifier'),
];

const EXT = new Set(['.ts', '.tsx', '.go']);

/** @param {string} dir @returns {string[]} */
function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const st = statSync(p);
    if (st.isDirectory()) {
      if (name === 'node_modules' || name === '.umi' || name === '.umi-production') continue;
      out.push(...walk(p));
    } else if (EXT.has(name.slice(name.lastIndexOf('.')))) {
      out.push(p);
    }
  }
  return out;
}

/** @type {{ file: string; line: number; text: string; hint: string }[]} */
const hits = [];

for (const dir of SCAN_DIRS) {
  let files;
  try {
    files = walk(dir);
  } catch {
    continue;
  }
  for (const file of files) {
    const content = readFileSync(file, 'utf8');
    const lines = content.split('\n');
    lines.forEach((line, i) => {
      const trimmed = line.trim();
      if (!trimmed) return;
      if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('import ')) return;
      if (trimmed.startsWith('/**') || trimmed.endsWith('*/')) return;
      if (trimmed.includes('TechnicalDetails') || trimmed.includes('JSON.stringify')) return;
      for (const rule of RULES) {
        if (!rule.pattern.test(line)) continue;
        if (rule.allow && rule.allow.test(line)) continue;
        hits.push({
          file: relative(ROOT, file).replace(/\\/g, '/'),
          line: i + 1,
          text: trimmed.slice(0, 120),
          hint: rule.hint,
        });
        break;
      }
    });
  }
}

if (hits.length === 0) {
  console.log('check-ui-copy: 未发现常见混排英文术语。');
  process.exit(0);
}

console.log(`check-ui-copy: 发现 ${hits.length} 处可能需中文化的用户文案：\n`);
for (const h of hits.slice(0, 80)) {
  console.log(`${h.file}:${h.line}  ${h.hint}`);
  console.log(`  ${h.text}\n`);
}
if (hits.length > 80) {
  console.log(`… 另有 ${hits.length - 80} 处，请本地运行完整扫描。\n`);
}
console.log('详见 docs/ui-copywriting.md');

process.exit(STRICT ? 1 : 0);
