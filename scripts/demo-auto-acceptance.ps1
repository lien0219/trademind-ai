# Phase R1.2-Auto - Demo automated acceptance orchestrator.
# Usage: .\scripts\demo-auto-acceptance.ps1 [-ApiBase http://127.0.0.1:8080] [-SkipBuild] [-SkipApiTests]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [switch]$SkipBuild,
    [switch]$SkipApiTests,
    [string]$ReportMd = "docs/DEMO_AUTO_ACCEPTANCE_REPORT.md",
    [string]$ReportJson = "docs/demo-auto-acceptance.json"
)

$ErrorActionPreference = "Continue"
$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot "backend"
$startedAt = (Get-Date).ToUniversalTime()
$steps = @()
$overallFailed = 0
$overallBlocked = 0

function Import-DotEnv {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        $key = $line.Substring(0, $idx).Trim()
        $val = $line.Substring($idx + 1).Trim()
        if ($val.StartsWith('"') -and $val.EndsWith('"')) { $val = $val.Substring(1, $val.Length - 2) }
        if (-not [string]::IsNullOrWhiteSpace($key) -and -not (Test-Path "env:$key")) {
            Set-Item -Path "env:$key" -Value $val
        }
    }
}

Import-DotEnv (Join-Path $repoRoot ".env")

function Add-Step {
    param(
        [string]$Name,
        [string]$Status,
        [int]$ExitCode = 0,
        [string]$Detail = ""
    )
    $script:steps += @{
        name       = $Name
        status     = $Status
        exitCode   = $ExitCode
        detail     = $Detail
        finishedAt = (Get-Date).ToUniversalTime().ToString("o")
    }
    if ($Status -eq "failed") { $script:overallFailed++ }
    if ($Status -eq "blocked") { $script:overallBlocked++ }
    Write-Host ("[{0}] {1} (exit {2}) {3}" -f $Status.ToUpper(), $Name, $ExitCode, $Detail)
}

function Run-Step {
    param(
        [string]$Name,
        [scriptblock]$Block,
        [int[]]$BlockedExitCodes = @(3)
    )
    Write-Host ""
    Write-Host "=== $Name ==="
    try {
        $result = & $Block
        if ($null -ne $result -and $result -is [int]) {
            $code = [int]$result
        } else {
            $code = $LASTEXITCODE
        }
        if ($null -eq $code) { $code = 0 }
        if ($BlockedExitCodes -contains $code) {
            Add-Step -Name $Name -Status "blocked" -ExitCode $code -Detail "blocked_by_config_or_credentials"
        } elseif ($code -ne 0) {
            Add-Step -Name $Name -Status "failed" -ExitCode $code
        } else {
            Add-Step -Name $Name -Status "passed" -ExitCode 0
        }
    } catch {
        Add-Step -Name $Name -Status "failed" -ExitCode 1 -Detail $_.Exception.Message
    }
}

function Test-BackendUp {
    try {
        $h = Invoke-RestMethod -Method Get -Uri "$ApiBase/health" -TimeoutSec 5
        return ($null -ne $h.data.status)
    } catch {
        return $false
    }
}

$backendUp = Test-BackendUp
$testEnv = @{
    apiBase   = $ApiBase
    backendUp = $backendUp
    appEnv    = $env:APP_ENV
    startedAt = $startedAt.ToString("o")
    hostname  = $env:COMPUTERNAME
    phase     = "Phase F8.1-Auto"
}

Write-Host "TradeMind Phase F8.1 Full-Project Demo Acceptance"
Write-Host "API: $ApiBase | Backend up: $backendUp"

$goPackages = @(
    "./...",
    "./internal/providers/platform/douyinshop/...",
    "./internal/modules/productpublish/...",
    "./internal/modules/ordersync/...",
    "./internal/modules/aiproducttext/...",
    "./internal/modules/aiproductimage/...",
    "./internal/modules/aiopsworkbench/...",
    "./internal/modules/taskcenter/...",
    "./internal/modules/customerchat/...",
    "./internal/modules/operationdashboard/...",
    "./internal/modules/order/...",
    "./internal/modules/inventory/...",
    "./internal/modules/configstatus/...",
    "./internal/pkg/adminperm/..."
)

Run-Step "go test regression" {
    Push-Location $backendDir
    $failures = @()
    foreach ($pkg in $goPackages) {
        Write-Host "  go test $pkg"
        $out = & go test $pkg 2>&1
        if ($LASTEXITCODE -ne 0) {
            $failures += @{ package = $pkg; output = ($out | Select-Object -Last 5) -join "`n" }
        }
    }
    Pop-Location
    if ($failures.Count -gt 0) {
        Write-Host ($failures | ConvertTo-Json -Depth 4)
        return 1
    }
    return 0
}

if (-not $SkipBuild) {
    Run-Step "go build backend" {
        Push-Location $backendDir
        New-Item -ItemType Directory -Path "tmp" -Force | Out-Null
        & go build -o tmp/server ./cmd/server/...
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }

    Run-Step "pnpm build:admin" {
        Push-Location $repoRoot
        & pnpm build:admin
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }

    Run-Step "git diff --check" {
        Push-Location $repoRoot
        & git diff --check
        $code = $LASTEXITCODE
        Pop-Location
        return $code
    }
} else {
    Add-Step -Name "go build backend" -Status "skipped" -Detail "-SkipBuild"
    Add-Step -Name "pnpm build:admin" -Status "skipped" -Detail "-SkipBuild"
    Add-Step -Name "git diff --check" -Status "skipped" -Detail "-SkipBuild"
}

Run-Step "check-ui-copy" {
    & node "$PSScriptRoot/check-ui-copy.mjs" --strict --report "docs/COPYWRITING_AUDIT.auto.md" --json "docs/global-status-copywriting-scan.json"
    return $LASTEXITCODE
}

Run-Step "demo-empty-state-scan" {
    & "$PSScriptRoot/demo-empty-state-scan.ps1" -OutFile "docs/demo-empty-state-scan.auto.json"
    return $LASTEXITCODE
}

Run-Step "demo-sensitive-confirm-scan" {
    & "$PSScriptRoot/demo-sensitive-confirm-scan.ps1" -OutFile "docs/demo-sensitive-confirm-scan.auto.json"
    return $LASTEXITCODE
}

Run-Step "security-release-check" {
    & "$PSScriptRoot/security-release-check.ps1" -OutFile "docs/SECURITY_RELEASE_CHECK.auto.md"
    return $LASTEXITCODE
}

Run-Step "check-doc-links" {
    & "$PSScriptRoot/check-doc-links.ps1" -OutFile "docs/DOCS_CONSISTENCY_CHECK.md"
    return $LASTEXITCODE
}

$apiStepNames = @(
    "demo-route-smoke", "seed-demo-data", "seed-demo-permissions",
    "demo-dashboard-smoke", "demo-rbac-smoke", "demo-order-inventory-customer-smoke",
    "ai-text-route-smoke", "ai-text-trial-run",
    "ai-image-route-smoke", "ai-image-trial-run", "publish-batch-perf", "ai-operation-workbench-perf"
)

if ($SkipApiTests -or -not $backendUp) {
    $reason = if ($SkipApiTests) { "-SkipApiTests" } else { "backend not reachable" }
    foreach ($n in $apiStepNames) {
        Add-Step -Name $n -Status "skipped" -Detail $reason
    }
} else {
    Run-Step "demo-route-smoke" {
        & "$PSScriptRoot/demo-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/demo-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "seed-demo-data" {
        & "$PSScriptRoot/seed-demo-data.ps1" -ApiBase $ApiBase -OutFile "docs/demo-dataset.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "seed-demo-permissions" {
        & "$PSScriptRoot/seed-demo-permissions.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-dashboard-smoke" {
        & "$PSScriptRoot/demo-dashboard-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-rbac-smoke" {
        & "$PSScriptRoot/demo-rbac-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "demo-order-inventory-customer-smoke" {
        & "$PSScriptRoot/demo-order-inventory-customer-smoke.ps1" -ApiBase $ApiBase
        return $LASTEXITCODE
    }
    Run-Step "ai-text-route-smoke" {
        & "$PSScriptRoot/ai-text-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/ai-text-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-text-trial-run" {
        & "$PSScriptRoot/ai-text-trial-run.ps1" -ApiBase $ApiBase -OutFile "docs/ai-text-trial-run.auto.json"
        return $LASTEXITCODE
    } -BlockedExitCodes @(3)
    Run-Step "ai-image-route-smoke" {
        & "$PSScriptRoot/ai-image-route-smoke.ps1" -ApiBase $ApiBase -OutFile "docs/ai-image-route-smoke.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-image-trial-run" {
        & "$PSScriptRoot/ai-image-trial-run.ps1" -ApiBase $ApiBase -OutFile "docs/ai-image-trial-run.auto.json"
        return $LASTEXITCODE
    } -BlockedExitCodes @(3)
    Run-Step "publish-batch-perf" {
        & "$PSScriptRoot/publish-batch-perf.ps1" -ApiBase $ApiBase -OutFile "docs/publish-batch-perf.auto.json"
        return $LASTEXITCODE
    }
    Run-Step "ai-operation-workbench-perf" {
        & "$PSScriptRoot/ai-operation-workbench-perf.ps1" -ApiBase $ApiBase -OutFile "docs/ai-operation-workbench-perf.auto.json"
        return $LASTEXITCODE
    }
}

$manualItems = @(
    "Real preprod SSH deployment",
    "Nginx / HTTPS",
    "Storage public access",
    "Preprod backup and rollback",
    "1366 / 1024 visual walkthrough",
    "Douyin real OAuth",
    "Douyin readonly E2E",
    "Douyin write E2E",
    "48-72h gray observation",
    "v0.1.0-demo tag final confirmation"
)

$automatableConclusion = if ($overallFailed -eq 0) {
    if ($overallBlocked -gt 0) { "passed_with_blocked" } else { "passed" }
} else { "failed" }

$finalStatus = @{
    release    = "MVP Demo Ready"
    tag        = "Tag pending"
    production = "Not Production Ready"
    douyin     = "Douyin Release Candidate"
}

$report = @{
    phase                 = "Phase F8.1-Auto"
    testEnvironment       = $testEnv
    startedAt             = $startedAt.ToString("o")
    finishedAt            = (Get-Date).ToUniversalTime().ToString("o")
    steps                 = $steps
    automatableConclusion = $automatableConclusion
    failedStepCount       = $overallFailed
    blockedStepCount      = $overallBlocked
    manualTestItems       = $manualItems
    finalStatus           = $finalStatus
    artifacts             = @{
        routeSmoke       = "docs/demo-route-smoke.auto.json"
        demoDataset      = "docs/demo-dataset.auto.json"
        fullProjectReport = "docs/demo-auto-acceptance.full-project.json"
        dashboardSmoke   = "docs/demo-dashboard-smoke.auto.json"
        rbacSmoke        = "docs/demo-rbac-smoke.auto.json"
        oicSmoke         = "docs/demo-order-inventory-customer-smoke.auto.json"
        emptyStateScan   = "docs/demo-empty-state-scan.auto.json"
        sensitiveScan    = "docs/demo-sensitive-confirm-scan.auto.json"
        globalStatusScan = "docs/global-status-copywriting-scan.json"
        aiTextTrial      = "docs/ai-text-trial-run.auto.json"
        aiImageTrial     = "docs/ai-image-trial-run.auto.json"
        publishBatchPerf = "docs/publish-batch-perf.auto.json"
        workbenchPerf    = "docs/ai-operation-workbench-perf.auto.json"
        copywritingAudit = "docs/COPYWRITING_AUDIT.auto.md"
        securityCheck    = "docs/SECURITY_RELEASE_CHECK.auto.md"
        docsConsistency  = "docs/DOCS_CONSISTENCY_CHECK.md"
    }
}

$dir = Split-Path -Parent $ReportJson
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 10 | Set-Content -Path $ReportJson -Encoding UTF8
$fullProjectJson = Join-Path $repoRoot "docs/demo-auto-acceptance.full-project.json"
$report | ConvertTo-Json -Depth 10 | Set-Content -Path $fullProjectJson -Encoding UTF8

$backendLabel = if ($backendUp) { "reachable" } else { "unreachable (API steps skipped)" }
$mdLines = New-Object System.Collections.Generic.List[string]
$mdLines.Add("# TradeMind Phase F8.1 Full-Project Demo Auto Acceptance Report")
$mdLines.Add("")
$mdLines.Add("> Generated: $($report.finishedAt)")
$mdLines.Add("> API: $ApiBase | Backend: $backendLabel")
$mdLines.Add("")
$mdLines.Add("## Phase")
$mdLines.Add("")
$mdLines.Add("**Phase F8.1-Auto** - Full-project demo smoke + static scans (not final manual acceptance)")
$mdLines.Add("")
$mdLines.Add("## Summary")
$mdLines.Add("")
$mdLines.Add("| Metric | Value |")
$mdLines.Add("| --- | --- |")
$mdLines.Add("| Conclusion | **$automatableConclusion** |")
$mdLines.Add("| Failed steps | $overallFailed |")
$mdLines.Add("| Blocked steps | $overallBlocked |")
$mdLines.Add("")
$mdLines.Add("## Step results")
$mdLines.Add("")
$mdLines.Add("| Step | Status | Exit | Detail |")
$mdLines.Add("| --- | --- | --- | --- |")
foreach ($s in $steps) {
    $mdLines.Add("| $($s.name) | $($s.status) | $($s.exitCode) | $($s.detail) |")
}
$mdLines.Add("")
$mdLines.Add("## Artifacts")
$mdLines.Add("")
$mdLines.Add("- [demo-route-smoke.auto.json](demo-route-smoke.auto.json)")
$mdLines.Add("- [demo-dataset.auto.json](demo-dataset.auto.json)")
$mdLines.Add("- [ai-text-trial-run.auto.json](ai-text-trial-run.auto.json)")
$mdLines.Add("- [ai-image-trial-run.auto.json](ai-image-trial-run.auto.json)")
$mdLines.Add("- [publish-batch-perf.auto.json](publish-batch-perf.auto.json)")
$mdLines.Add("- [ai-operation-workbench-perf.auto.json](ai-operation-workbench-perf.auto.json)")
$mdLines.Add("- [COPYWRITING_AUDIT.auto.md](COPYWRITING_AUDIT.auto.md)")
$mdLines.Add("- [SECURITY_RELEASE_CHECK.auto.md](SECURITY_RELEASE_CHECK.auto.md)")
$mdLines.Add("- [DOCS_CONSISTENCY_CHECK.md](DOCS_CONSISTENCY_CHECK.md)")
$mdLines.Add("")
$mdLines.Add("## Manual test checklist (out of scope for automation)")
$mdLines.Add("")
foreach ($m in $manualItems) {
    $mdLines.Add("- [ ] $m")
}
$mdLines.Add("")
$mdLines.Add("## Final status")
$mdLines.Add("")
$mdLines.Add('```text')
$mdLines.Add("MVP Demo Ready")
$mdLines.Add("Tag pending")
$mdLines.Add("Not Production Ready")
$mdLines.Add("Douyin Release Candidate")
$mdLines.Add('```')
$mdLines.Add("")
$mdLines.Add("No v0.1.0-demo tag in this phase. No real Douyin E2E. No production gray release.")

$mdDir = Split-Path -Parent $ReportMd
if ($mdDir -and -not (Test-Path $mdDir)) { New-Item -ItemType Directory -Path $mdDir -Force | Out-Null }
$mdLines -join "`n" | Set-Content -Path $ReportMd -Encoding UTF8
$fullProjectMd = Join-Path $repoRoot "docs/DEMO_AUTO_ACCEPTANCE_FULL_PROJECT_REPORT.md"
$mdLines -join "`n" | Set-Content -Path $fullProjectMd -Encoding UTF8

Write-Host ""
Write-Host "=== Summary ==="
Write-Host "Automatable conclusion: $automatableConclusion"
Write-Host "Failed: $overallFailed | Blocked: $overallBlocked"
Write-Host "Wrote $ReportMd"
Write-Host "Wrote $ReportJson"

if ($overallFailed -gt 0) { exit 1 }
exit 0
