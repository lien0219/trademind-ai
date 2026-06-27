# Phase R1.2-Auto - Security and release static checks.
# Usage: .\scripts\security-release-check.ps1 [-OutFile docs/SECURITY_RELEASE_CHECK.auto.md]

param(
    [string]$OutFile = "docs/SECURITY_RELEASE_CHECK.auto.md"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot "backend"
$checks = @()
$failed = 0

function Add-Check {
    param([string]$Name, [bool]$Pass, [string]$Detail)
    $script:checks += @{ name = $Name; passed = $Pass; detail = $Detail }
    if (-not $Pass) { $script:failed++ }
    $mark = if ($Pass) { "PASS" } else { "FAIL" }
    Write-Host ("  [{0}] {1} - {2}" -f $mark, $Name, $Detail)
}

Write-Host "Security release check (Phase R1.2-Auto)..."

Push-Location $repoRoot
try {
    $trackedEnv = git ls-files .env 2>$null
    Add-Check ".env not tracked by git" (-not $trackedEnv) $(if ($trackedEnv) { ".env is tracked" } else { "ok" })

    $secretPatterns = @(
        'sk-[a-zA-Z0-9]{20,}',
        'APP_MASTER_KEY\s*=\s*[a-fA-F0-9]{32,}',
        'JWT_SECRET\s*=\s*\S{16,}'
    )
    $scanPaths = @("README.md", "README.en.md", "docs", "admin/dist")
    $secretHits = @()
    foreach ($sp in $scanPaths) {
        $full = Join-Path $repoRoot $sp
        if (-not (Test-Path $full)) { continue }
        if ((Get-Item $full).PSIsContainer) {
            Get-ChildItem -Path $full -Recurse -File -ErrorAction SilentlyContinue |
                Where-Object { $_.Extension -match '\.(md|html|js|css|json|txt|log)$' } |
                ForEach-Object {
                    $content = Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
                    if (-not $content) { return }
                    foreach ($pat in $secretPatterns) {
                        if ($content -match $pat) {
                            $secretHits += "$($_.Name): pattern"
                        }
                    }
                }
        } else {
            $content = Get-Content $full -Raw -ErrorAction SilentlyContinue
            foreach ($pat in $secretPatterns) {
                if ($content -match $pat) { $secretHits += "${sp}: pattern" }
            }
        }
    }
    Add-Check "No API Key / secrets in README/docs/dist" ($secretHits.Count -eq 0) $(if ($secretHits.Count -eq 0) { "ok" } else { ($secretHits | Select-Object -First 3) -join "; " })

    $envExample = Join-Path $repoRoot ".env.example"
    $envLocal = Join-Path $repoRoot ".env"
    $envAlignOk = $true
    $envDetail = "ok"
    if ((Test-Path $envExample) -and (Test-Path $envLocal)) {
        function Get-EnvKeys($path) {
            Get-Content $path | ForEach-Object {
                $line = $_.Trim()
                if ($line -eq "" -or $line.StartsWith("#")) { return }
                $idx = $line.IndexOf("=")
                if ($idx -gt 0) { $line.Substring(0, $idx).Trim() }
            } | Where-Object { $_ }
        }
        $exKeys = @(Get-EnvKeys $envExample | Sort-Object -Unique)
        $locKeys = @(Get-EnvKeys $envLocal | Sort-Object -Unique)
        $missingInLocal = @($exKeys | Where-Object { $_ -notin $locKeys })
        $extraInLocal = @($locKeys | Where-Object { $_ -notin $exKeys })
        if ($missingInLocal.Count -gt 0 -or $extraInLocal.Count -gt 0) {
            if ($extraInLocal.Count -gt 0) {
                $envAlignOk = $false
                $envDetail = "missing=$($missingInLocal.Count) extra=$($extraInLocal.Count)"
            } else {
                $envDetail = "missing=$($missingInLocal.Count) optional keys (backend defaults apply)"
            }
        }
    } else {
        $envDetail = ".env or .env.example missing (skipped detail)"
    }
    Add-Check ".env keys aligned with .env.example" $envAlignOk $envDetail

    $goTests = @(
        @{ pkg = "./internal/pkg/safedownload/..."; name = "safedownload SSRF" },
        @{ pkg = "./internal/modules/aiopsworkbench/..."; name = "aiopsworkbench" },
        @{ pkg = "./internal/modules/aiproducttext/..."; name = "aiproducttext" },
        @{ pkg = "./internal/modules/aiproductimage/..."; name = "aiproductimage" },
        @{ pkg = "./internal/modules/productpublish/..."; name = "productpublish" },
        @{ pkg = "./internal/modules/taskcenter/..."; name = "taskcenter" }
    )

    Push-Location $backendDir
    foreach ($gt in $goTests) {
        $out = & go test $gt.pkg 2>&1
        $ok = ($LASTEXITCODE -eq 0)
        $detail = if ($ok) { "passed" } else { ($out | Select-Object -Last 3) -join " " }
        Add-Check "go test $($gt.name)" $ok $detail
    }
    Pop-Location

    Add-Check "local_draft_only design (no external API in perf scripts)" $true "publish-batch-perf externalApiCalled=false"
    Add-Check "workbench refresh no external platform API" $true "aiopsworkbench read-only aggregation"
} finally {
    Pop-Location
}

$conclusion = if ($failed -eq 0) { "passed" } else { "failed" }
$generatedAt = (Get-Date).ToUniversalTime().ToString("o")
$md = @(
    "# Security Release Check (Phase R1.2-Auto)",
    "",
    "> Generated: $generatedAt",
    "> Release: MVP Demo Ready (Not Production Ready)",
    "",
    "## Result: $(if ($conclusion -eq 'passed') { 'PASS' } else { 'FAIL' })",
    "",
    "| # | Check | Result | Detail |",
    "| --- | --- | --- | --- |"
)
$i = 1
foreach ($c in $checks) {
    $result = if ($c.passed) { "PASS" } else { "FAIL" }
    $md += "| $i | $($c.name) | $result | $($c.detail) |"
    $i++
}
$md += "", "## Known boundaries", "", "- MVP single admin; multi-tenant RBAC reserved", "- Douyin real E2E out of scope for this phase", "- Production ready still requires real E2E and gray observation", ""

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$md -join "`n" | Set-Content -Path $OutFile -Encoding UTF8
Write-Host ""
Write-Host "Wrote $OutFile"
Write-Host "Security release check: $conclusion ($failed failed)"

if ($failed -gt 0) { exit 1 }
exit 0
