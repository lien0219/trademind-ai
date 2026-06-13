# Aggregate Douyin E2E JSON reports into markdown.
$ErrorActionPreference = "Stop"

$ReportDir = if ($env:DOUYIN_E2E_REPORT_DIR) { $env:DOUYIN_E2E_REPORT_DIR } else { "./tmp/douyin-e2e" }
$OutMd = if ($env:DOUYIN_E2E_REPORT_MD) { $env:DOUYIN_E2E_REPORT_MD } else { Join-Path $ReportDir "douyin-e2e-report.md" }
$GitSha = try { (git rev-parse --short HEAD) } catch { "unknown" }
$Ts = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

$blocked = "是"
$releaseStatus = "Release Candidate"
$grayOk = "否"
$rollbackDrill = "environment_simulation_only"
$ciRace = "see .github/workflows/go.yml backend-race job"

if (Test-Path $ReportDir) {
    $latest = Get-ChildItem -Path $ReportDir -Filter "preflight-*.json" -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if ($latest) {
        $raw = Get-Content -Raw -Path $latest.FullName
        if ($raw -match '"blockedByRealCredentials"\s*:\s*false') { $blocked = "否" }
    }
}

$realE2e = if ($blocked -eq "是") { "blocked_by_real_credentials" } else { "passed_or_partial" }

$content = @"
# 抖店 E2E 验收报告（脚本生成）

> 生成时间（UTC）：$Ts  
> Git SHA：$GitSha  
> 发布状态：$releaseStatus  
> ``blocked_by_real_credentials``：$blocked

## 发布结论（Phase 10.4）

| 字段 | 值 |
| --- | --- |
| release_candidate_status | $releaseStatus |
| real_e2e_status | $realE2e |
| gray_release_approved | $grayOk |
| rollback_drill_status | $rollbackDrill |
| ci_backend_race | $ciRace |
| production_available | 否 |

## 脚本工件

目录：``$ReportDir``

## 下一步

- 配置真实抖店凭证后重跑 ``scripts/douyin-e2e-*.ps1``
- 完整模板：docs/DOUYIN_E2E_REPORT_TEMPLATE.md
- 灰度门禁：docs/DOUYIN_RELEASE_GATE.md

"@

New-Item -ItemType Directory -Force -Path (Split-Path $OutMd -Parent) | Out-Null
Set-Content -Path $OutMd -Value $content -Encoding UTF8
Write-Host "report: $OutMd"
if ($blocked -eq "是") {
    Write-Error "blocked_by_real_credentials"
    exit 3
}
Write-Host "ok: report generated"
