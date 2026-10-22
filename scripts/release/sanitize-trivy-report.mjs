import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const severityNames = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN'];
const safeValuePattern = /^[A-Za-z0-9@._:+/~-]{1,128}$/u;
const vulnerabilityIdPattern = /^(?:CVE-\d{4}-\d{4,}|GHSA-[A-Za-z0-9-]{8,}|GO-\d{4}-\d{4,})$/u;
const allowedOutcomes = new Set(['success', 'failure', 'cancelled', 'skipped']);
const maxItems = 1000;

function safeValue(value) {
  return typeof value === 'string' && safeValuePattern.test(value) ? value : undefined;
}

function severity(value) {
  return typeof value === 'string' && severityNames.includes(value.toUpperCase())
    ? value.toUpperCase()
    : 'UNKNOWN';
}

function emptyReport(status, scannerOutcome) {
  return {
    schemaVersion: 1,
    status,
    scannerOutcome,
    vulnerabilities: {
      total: 0,
      severities: Object.fromEntries(severityNames.map((name) => [name, 0])),
      items: [],
      truncated: false,
    },
    secretFindings: { total: 0 },
  };
}

/** Return a strict allow-list projection; never copy Trivy free-form fields. */
export function sanitizeTrivyReport(report, scannerOutcome = 'unknown') {
  const outcome = allowedOutcomes.has(scannerOutcome) ? scannerOutcome : 'unknown';
  if (!report || typeof report !== 'object' || Array.isArray(report)) {
    return emptyReport('unavailable', outcome);
  }

  if (!Array.isArray(report.Results)) {
    const isSuccessfulEmptyImageReport = outcome === 'success'
      && !Object.hasOwn(report, 'Results')
      && report.SchemaVersion === 2
      && report.ArtifactType === 'container_image'
      && typeof report.ArtifactName === 'string'
      && report.ArtifactName.trim().length > 0;
    if (!isSuccessfulEmptyImageReport) return emptyReport('unavailable', outcome);
    return emptyReport('complete', outcome);
  }

  const sanitized = emptyReport('complete', outcome);
  const uniqueItems = new Map();
  let truncated = false;

  for (const result of report.Results) {
    if (!result || typeof result !== 'object') continue;

    if (Array.isArray(result.Secrets)) {
      sanitized.secretFindings.total += result.Secrets.length;
    }

    if (!Array.isArray(result.Vulnerabilities)) continue;
    for (const finding of result.Vulnerabilities) {
      sanitized.vulnerabilities.total += 1;
      const findingSeverity = severity(finding?.Severity);
      sanitized.vulnerabilities.severities[findingSeverity] += 1;

      if (!finding || typeof finding !== 'object') continue;
      const id = safeValue(finding.VulnerabilityID);
      const packageName = safeValue(finding.PkgName);
      const installedVersion = safeValue(finding.InstalledVersion);
      if (!id || !vulnerabilityIdPattern.test(id) || !packageName || !installedVersion) continue;

      const item = {
        id,
        package: packageName,
        installedVersion,
        severity: findingSeverity,
      };
      const fixedVersion = safeValue(finding.FixedVersion);
      const target = safeValue(result.Target);
      const targetType = safeValue(result.Type);
      const targetClass = safeValue(result.Class);
      if (fixedVersion) item.fixedVersion = fixedVersion;
      if (target) item.target = target;
      if (targetType) item.targetType = targetType;
      if (targetClass) item.targetClass = targetClass;
      const key = JSON.stringify(item);
      if (uniqueItems.has(key)) continue;
      if (uniqueItems.size >= maxItems) {
        truncated = true;
        continue;
      }
      uniqueItems.set(key, item);
    }
  }

  sanitized.vulnerabilities.items = [...uniqueItems.values()];
  sanitized.vulnerabilities.truncated = truncated;
  return sanitized;
}

function sanitizeFile(inputPath, outputPath, outcome) {
  let sanitized;
  try {
    sanitized = sanitizeTrivyReport(JSON.parse(readFileSync(inputPath, 'utf8')), outcome);
  } catch {
    sanitized = emptyReport(
      'unavailable',
      allowedOutcomes.has(outcome) ? outcome : 'unknown',
    );
  }

  writeFileSync(outputPath, `${JSON.stringify(sanitized, null, 2)}\n`, { mode: 0o600 });
  return sanitized.status === 'complete';
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const [, , inputPath, outputPath, outcome = 'unknown'] = process.argv;
  if (!inputPath || !outputPath) process.exit(2);
  if (!sanitizeFile(inputPath, outputPath, outcome)) process.exit(1);
}
