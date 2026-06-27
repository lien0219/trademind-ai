# AI product image route smoke test — verifies /health and /api/v1/products/ai-images/* are registered (not 404).
# Usage: .\scripts\ai-image-route-smoke.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/ai-image-route-smoke.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$OutFile = "docs/ai-image-route-smoke.json"
)

$ErrorActionPreference = "Stop"

function Get-HttpStatus {
    param([string]$Method, [string]$Url)
    $code = & curl.exe -s -o NUL -w "%{http_code}" -X $Method $Url 2>$null
    if ($code -match '^\d{3}$') { return [int]$code }
    throw "Failed to probe $Method $Url"
}

$routes = @(
    @{ Method = "GET";  Path = "/health"; ExpectNot404 = $true },
    @{ Method = "GET";  Path = "/api/v1/products/ai-images/batches"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches/check"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches"; ExpectNot404 = $true },
    @{ Method = "GET";  Path = "/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/retry-failed"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/cancel-pending"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/apply-selected"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/undo-applied"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/regenerate"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/apply"; ExpectNot404 = $true },
    @{ Method = "POST"; Path = "/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/reject"; ExpectNot404 = $true }
)

Write-Host "AI image route smoke test against $ApiBase"

$healthJson = $null
$healthTs = $null
try {
    $healthJson = Invoke-RestMethod -Method Get -Uri "$ApiBase/health"
    $healthTs = $healthJson.data.timestamp
} catch {
    Write-Error "Health check failed: $($_.Exception.Message)"
    exit 1
}

$results = @()
$failed = 0
foreach ($r in $routes) {
    $url = "$ApiBase$($r.Path)"
    $status = Get-HttpStatus -Method $r.Method -Url $url
    $ok = ($status -ne 404)
    if (-not $ok) { $failed++ }
    $note = if ($status -eq 401 -or $status -eq 403) { "auth_required_ok" } elseif ($status -eq 200) { "ok" } else { "status_$status" }
    Write-Host ("  {0,-6} {1,-80} -> {2} {3}" -f $r.Method, $r.Path, $status, $(if ($ok) { "PASS" } else { "FAIL" }))
    $results += @{
        method = $r.Method
        path = $r.Path
        statusCode = $status
        not404 = $ok
        note = $note
    }
}

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase = $ApiBase
    healthTimestamp = $healthTs
    healthStatus = $healthJson.data.status
    appEnv = $healthJson.data.appEnv
    imageQueue = $healthJson.data.imageQueue
    routeCount = $routes.Count
    failed404Count = $failed
    passed = ($failed -eq 0)
    routes = $results
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 6 | Set-Content -Path $OutFile -Encoding UTF8

Write-Host ""
Write-Host "Health timestamp: $healthTs"
Write-Host "App env: $($healthJson.data.appEnv)"
Write-Host "Image queue enabled: $($healthJson.data.imageQueue.enabled)"
Write-Host "Wrote $OutFile"
if ($failed -gt 0) {
    Write-Error "$failed route(s) returned 404"
    exit 2
}
Write-Host "All routes registered (no 404). Smoke test PASSED."
exit 0
