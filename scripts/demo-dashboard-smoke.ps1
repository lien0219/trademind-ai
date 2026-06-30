# F7 Dashboard API smoke — KPI + todos must not 500.
param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$OutFile = "docs/demo-dashboard-smoke.auto.json"
)

$ErrorActionPreference = "Continue"
$ApiV1 = "$ApiBase/api/v1"
$repoRoot = Split-Path -Parent $PSScriptRoot

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
        if (-not [string]::IsNullOrWhiteSpace($key) -and -not (Test-Path "env:$key")) { Set-Item -Path "env:$key" -Value $val }
    }
}
Import-DotEnv (Join-Path $repoRoot ".env")

$account = $env:ADMIN_BOOTSTRAP_EMAIL
$password = $env:ADMIN_BOOTSTRAP_PASSWORD
if (-not $account -or -not $password) { Write-Error "Set ADMIN_BOOTSTRAP_EMAIL/PASSWORD"; exit 1 }

$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $account; password = $password } | ConvertTo-Json)
$token = $login.data.token
$headers = @{ Authorization = "Bearer $token"; Accept = "application/json" }

$endpoints = @(
    "/dashboard/overview",
    "/dashboard/todos",
    "/dashboard/health",
    "/dashboard/product-operations",
    "/settings/config-status"
)
$results = @()
$failed = 0
foreach ($ep in $endpoints) {
    $status = "passed"
    $code = 0
    $detail = ""
    try {
        $r = Invoke-WebRequest -Method Get -Uri "$ApiV1$ep" -Headers $headers -TimeoutSec 15
        $code = [int]$r.StatusCode
        if ($code -ge 500) { $status = "failed"; $failed++ }
    } catch {
        $status = "failed"
        $failed++
        $detail = $_.Exception.Message
        if ($_.Exception.Response) { $code = [int]$Exception.Response.StatusCode.value__ }
    }
    $results += @{ endpoint = $ep; status = $status; httpCode = $code; detail = $detail }
}

$out = @{ phase = "F7"; generatedAt = (Get-Date).ToUniversalTime().ToString("o"); results = $results; failed = $failed }
$path = Join-Path $repoRoot $OutFile
$out | ConvertTo-Json -Depth 6 | Set-Content -Path $path -Encoding UTF8
Write-Host "Wrote $path (failed=$failed)"
exit $(if ($failed -gt 0) { 1 } else { 0 })
