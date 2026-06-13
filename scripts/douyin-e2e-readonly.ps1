# Douyin E2E readonly chain — categories, task-center, operation logs (no platform writes).
$ErrorActionPreference = "Stop"

$ApiBase = if ($env:TRADEMIND_API_BASE) { $env:TRADEMIND_API_BASE.TrimEnd("/") } else { "http://127.0.0.1:8080" }
$ApiV1 = "$ApiBase/api/v1"
$Account = $env:TRADEMIND_ADMIN_ACCOUNT
$Password = $env:TRADEMIND_ADMIN_PASSWORD
$ReportDir = if ($env:DOUYIN_E2E_REPORT_DIR) { $env:DOUYIN_E2E_REPORT_DIR } else { "./tmp/douyin-e2e" }

New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$Out = Join-Path $ReportDir "readonly-$Ts.json"

function Exit-Blocked {
    Write-Error "blocked_by_real_credentials"
    exit 3
}

function Invoke-ApiJson {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token)
    $headers = @{ "Content-Type" = "application/json"; "Authorization" = "Bearer $Token" }
    if ($Body) { return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers -Body $Body }
    return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers
}

if ([string]::IsNullOrWhiteSpace($Account) -or [string]::IsNullOrWhiteSpace($Password)) { Exit-Blocked }

$loginBody = @{ account = $Account; password = $Password } | ConvertTo-Json
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" -Body $loginBody
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

$preflight = Invoke-ApiJson -Method Post -Url "$ApiV1/platform/douyin/production-preflight" -Body '{"liveTest":true}' -Token $token
if ($preflight.data.blockedByRealCredentials -eq $true) { Exit-Blocked }

$catStats = Invoke-ApiJson -Method Get -Url "$ApiV1/platform/douyin/categories/stats" -Token $token
$tcSummary = Invoke-ApiJson -Method Get -Url "$ApiV1/task-center/summary" -Token $token
$tcFailures = Invoke-ApiJson -Method Get -Url "$ApiV1/task-center/failures?page=1&pageSize=5&keyword=DOUYIN" -Token $token
$opLogs = Invoke-ApiJson -Method Get -Url "$ApiV1/operation-logs?page=1&pageSize=10&action=douyin" -Token $token
$dash = Invoke-ApiJson -Method Get -Url "$ApiV1/dashboard/product-operations" -Token $token

$payload = @{
    preflight = $preflight.data
    categoryStats = $catStats.data
    taskCenterSummary = $tcSummary.data
    recentDouyinFailures = $tcFailures.data
    douyinOperationLogs = $opLogs.data
    dashboard = $dash.data
} | ConvertTo-Json -Depth 20
Set-Content -Path $Out -Value $payload -Encoding UTF8
Write-Host "report: $Out"
Write-Host "ok: readonly E2E probes completed"
