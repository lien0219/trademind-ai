#!/usr/bin/env node
/**
 * 扫描用户可见路径中的常见英文/技术混排与内部码直出。
 * 用法: node scripts/check-ui-copy.mjs [--strict] [--report docs/COPYWRITING_AUDIT.auto.md]
 */
import { readFileSync, readdirSync, statSync, writeFileSync, mkdirSync } from 'fs';
import { join, relative, dirname } from 'path';

const ROOT = join(import.meta.dirname, '..');
const STRICT = process.argv.includes('--strict');
const reportIdx = process.argv.indexOf('--report');
const REPORT_FILE = reportIdx >= 0 ? process.argv[reportIdx + 1] : null;

const INTERNAL_SCAN_DIRS = [
  join(ROOT, 'admin/src/pages'),
  join(ROOT, 'admin/src/components'),
];

/** 代码逻辑 / 映射层 — 不算 UI 主文案直出 */
const INTERNAL_CODE_ALLOW =
  /===|!==|dataIndex:|rowKey=|rowKey:|operationTypes|operationType:|filter\(|includes\(|\.status|value:\s*['"`]|label:\s*['"`]|text:\s*['"`]|color:|OP_LABEL|statusTag|getStatus|publishLabels|aiProductText|aiProductImage|TechnicalDetails|TaskJsonBlock|copywriting|\/\*\*|\/\/|params\.|detail\.batch|bulkOp|selectedOps|opChoice|form\.|API|ConvertToJson|type\s+\w+|来源类型|来源编号|问题代码|内容快照|qualityWarnings\?|qualityWarnings\.|qualityWarnings\[|expectedUpdatedAt:|sourceType:|sourceId:|postOrderException|deleteOrderException|relatedResource|partial_success:|partial_success,|\['partial_success'|StatusTag\.tsx|addList\(|undoProduct|undoAiDescription|await\s+/i;

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
  { pattern: /\bblocked_by_/i, hint: '→ 中文说明', allow: /blocked_by_[a-z_]+:\s*['"`]/i },
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
];

const ALL_RULES = RULES;

/** @type {{ file: string; line: number; text: string; hint: string; term: string }[]} */
const internalHits = [];

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

const INTERNAL_TERMS = [
  'pending_review',
  'partial_success',
  'sourceSnapshotHash',
  'expectedUpdatedAt',
  'sourceType',
  'sourceId',
  'target_key',
  'batch_id',
  'real_draft_create',
  'local_draft_only',
  'qualityWarnings',
  'blocked_by_real_credentials',
  'blocked_by_provider_config',
  'unsupported_by_provider',
];

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
      for (const rule of ALL_RULES) {
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

for (const dir of INTERNAL_SCAN_DIRS) {
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
      if (INTERNAL_CODE_ALLOW.test(line)) return;
      for (const term of INTERNAL_TERMS) {
        if (!line.includes(term)) continue;
        internalHits.push({
          file: relative(ROOT, file).replace(/\\/g, '/'),
          line: i + 1,
          text: trimmed.slice(0, 120),
          hint: `内部码 "${term}" 可能直出 UI 主文案`,
          term,
        });
        break;
      }
    });
  }
}

const allHits = [...hits, ...internalHits.map(({ term, ...rest }) => rest)];

if (allHits.length === 0) {
  console.log('check-ui-copy: 未发现常见混排英文术语或内部码直出。');
  if (REPORT_FILE) {
    writeReport([]);
  }
  process.exit(0);
}

console.log(`check-ui-copy: 发现 ${allHits.length} 处可能需中文化的用户文案：\n`);
for (const h of allHits.slice(0, 80)) {
  console.log(`${h.file}:${h.line}  ${h.hint}`);
  console.log(`  ${h.text}\n`);
}
if (allHits.length > 80) {
  console.log(`… 另有 ${allHits.length - 80} 处，请本地运行完整扫描。\n`);
}
console.log('详见 docs/ui-copywriting.md');

if (REPORT_FILE) {
  writeReport(allHits);
}

process.exit(STRICT ? 1 : 0);

/** @param {typeof hits} items */
function writeReport(items) {
  const dir = dirname(REPORT_FILE);
  try {
    mkdirSync(dir, { recursive: true });
  } catch {
    /* exists */
  }
  const lines = [
    '# Demo Release 中文文案自动审计（Phase R1.2-Auto）',
    '',
    `> 生成时间：${new Date().toISOString()}`,
    `> 工具：\`node scripts/check-ui-copy.mjs --strict --report\``,
    '',
    `## 结论：${items.length === 0 ? '✅ 通过' : '❌ 未通过'}`,
    '',
    `共发现 **${items.length}** 处可能直出内部码/英文术语。`,
    '',
  ];
  if (items.length === 0) {
    lines.push('- UI 主路径无 P1 级内部码直出', '- 技术详情折叠区允许出现 JSON 键名', '');
  } else {
    lines.push('## 命中明细', '', '| 文件 | 行 | 提示 | 上下文 |', '| --- | --- | --- | --- |');
    for (const h of items.slice(0, 200)) {
      lines.push(`| \`${h.file}\` | ${h.line} | ${h.hint} | \`${h.text.replace(/\|/g, '\\|')}\` |`);
    }
    if (items.length > 200) {
      lines.push('', `… 另有 ${items.length - 200} 处未列出。`);
    }
  }
  lines.push('', '## 白名单说明', '', '- `TechnicalDetails` / `TaskJsonBlock` 折叠区允许 JSON 键名', '- `constants/*Labels*` 映射文件允许内部码作为 key', '- 文档、测试、脚本路径不在扫描范围', '');
  writeFileSync(REPORT_FILE, lines.join('\n'), 'utf8');
  console.log(`Wrote ${REPORT_FILE}`);
}
