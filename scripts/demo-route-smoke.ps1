# Phase R1 — Unified demo route smoke test (health + core MVP APIs).
# Usage: .\scripts\demo-route-smoke.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/demo-route-smoke.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/demo-route-smoke.json"
)

$ErrorActionPreference = "Stop"
$ApiV1 = "$ApiBase/api/v1"

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

$repoRoot = Split-Path -Parent $PSScriptRoot
Import-DotEnv (Join-Path $repoRoot ".env")
if (-not $Account) { $Account = $env:ADMIN_BOOTSTRAP_EMAIL }
if (-not $Password) { $Password = $env:ADMIN_BOOTSTRAP_PASSWORD }

function Get-HttpStatusTimed {
    param([string]$Method, [string]$Url, [string]$Token = $null)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $args = @("-s", "-o", "NUL", "-w", "%{http_code}", "-X", $Method)
    if ($Token) { $args += @("-H", "Authorization: Bearer $Token") }
    $args += $Url
    $code = & curl.exe @args 2>$null
    $sw.Stop()
    if ($code -match '^\d{3}$') {
        return @{ statusCode = [int]$code; durationMs = [math]::Round($sw.Elapsed.TotalMilliseconds, 1) }
    }
    throw "Failed to probe $Method $Url"
}

$routes = @(
    @{ Method = "GET"; Path = "/health"; Auth = $false },
    @{ Method = "GET"; Path = "/api/v1/products"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/products/ai-text/batches"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/products/ai-images/batches"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/product-publish/batches"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/ai/operation-workbench/summary"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/ai/operation-workbench/todos?page=1&pageSize=50"; Auth = $true },
    @{ Method = "GET"; Path = "/api/v1/task-center/failures"; Auth = $true }
)

Write-Host "Demo route smoke test against $ApiBase"

$healthJson = $null
try {
    $healthJson = Invoke-RestMethod -Method Get -Uri "$ApiBase/health"
} catch {
    Write-Error "Health check failed: $($_.Exception.Message)"
    exit 1
}

$token = $null
if ($Account -and $Password) {
    try {
        $login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
            -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
        $token = $login.data.token
    } catch {
        Write-Warning "Login failed; authenticated routes will only be checked unauthenticated."
    }
}

$results = @()
$failed404 = 0
$failed500 = 0
$authIssues = 0

foreach ($r in $routes) {
    $url = if ($r.Path.StartsWith("/api/")) { "$ApiBase$($r.Path)" } else { "$ApiBase$($r.Path)" }
    $useToken = if ($r.Auth) { $token } else { $null }
    $probe = Get-HttpStatusTimed -Method $r.Method -Url $url -Token $useToken
    $status = $probe.statusCode
    $not404 = ($status -ne 404)
    $not500 = ($status -ne 500)
    if (-not $not404) { $failed404++ }
    if (-not $not500) { $failed500++ }

    $note = "status_$status"
    if ($r.Auth -and -not $token) {
        $note = "login_skipped"
    } elseif ($r.Auth -and ($status -eq 401 -or $status -eq 403)) {
        $note = "auth_required_ok"
        $authIssues++
    } elseif ($r.Auth -and $status -eq 200) {
        $note = "authenticated_ok"
    } elseif (-not $r.Auth -and $status -eq 200) {
        $note = "ok"
    }

    $pass = $not404 -and $not500 -and ($note -ne "login_skipped")
    Write-Host ("  {0,-6} {1,-60} -> {2} ({3}ms) {4}" -f $r.Method, $r.Path, $status, $probe.durationMs, $(if ($pass) { "PASS" } else { "FAIL" }))
    $results += @{
        method      = $r.Method
        path        = $r.Path
        auth        = $r.Auth
        statusCode  = $status
        durationMs  = $probe.durationMs
        not404      = $not404
        not500      = $not500
        note        = $note
    }
}

# Unauthenticated check on protected routes
$unauthResults = @()
foreach ($r in ($routes | Where-Object { $_.Auth })) {
    $url = "$ApiBase$($r.Path)"
    $probe = Get-HttpStatusTimed -Method $r.Method -Url $url
    $status = $probe.statusCode
    $ok = ($status -eq 401 -or $status -eq 403)
    $unauthResults += @{
        path       = $r.Path
        statusCode = $status
        durationMs = $probe.durationMs
        authGateOk = $ok
    }
}

$report = @{
    generatedAt       = (Get-Date).ToUniversalTime().ToString("o")
    apiBase           = $ApiBase
    healthStatus      = $healthJson.data.status
    healthTimestamp   = $healthJson.data.timestamp
    appEnv            = $healthJson.data.appEnv
    loggedIn          = [bool]$token
    routeCount        = $routes.Count
    failed404Count    = $failed404
    failed500Count    = $failed500
    passed            = ($failed404 -eq 0 -and $failed500 -eq 0 -and [bool]$token)
    routes            = $results
    unauthenticated   = $unauthResults
    note              = "task-center path is /api/v1/task-center/failures (not taskcenter)"
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 8 | Set-Content -Path $OutFile -Encoding UTF8

Write-Host ""
Write-Host "Wrote $OutFile"
if ($failed404 -gt 0) {
    Write-Error "$failed404 route(s) returned 404"
    exit 2
}
if ($failed500 -gt 0) {
    Write-Error "$failed500 route(s) returned 500"
    exit 4
}
if (-not $token) {
    Write-Error "Login required for full smoke pass; set ADMIN_BOOTSTRAP_EMAIL/PASSWORD"
    exit 3
}
Write-Host "Demo route smoke test PASSED."
exit 0
