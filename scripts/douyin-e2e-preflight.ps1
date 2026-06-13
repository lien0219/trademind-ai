# Douyin E2E preflight — config, health, production-preflight, runtime-status.
# Exit 3 + "blocked_by_real_credentials" when App Key/Secret or authorized shop missing.
$ErrorActionPreference = "Stop"

$ApiBase = if ($env:TRADEMIND_API_BASE) { $env:TRADEMIND_API_BASE.TrimEnd("/") } else { "http://127.0.0.1:8080" }
$ApiV1 = "$ApiBase/api/v1"
$Account = $env:TRADEMIND_ADMIN_ACCOUNT
$Password = $env:TRADEMIND_ADMIN_PASSWORD
$LiveTest = $env:DOUYIN_E2E_LIVE_TEST
$ReportDir = if ($env:DOUYIN_E2E_REPORT_DIR) { $env:DOUYIN_E2E_REPORT_DIR } else { "./tmp/douyin-e2e" }

New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$Out = Join-Path $ReportDir "preflight-$Ts.json"

function Exit-Blocked {
    Write-Error "blocked_by_real_credentials"
    exit 3
}

function Invoke-ApiJson {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Body = $null,
        [string]$Token = $null
    )
    $headers = @{ "Content-Type" = "application/json" }
    if ($Token) { $headers["Authorization"] = "Bearer $Token" }
    if ($Body) {
        return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers -Body $Body
    }
    return Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers
}

try {
    $health = Invoke-RestMethod -Method Get -Uri "$ApiBase/health"
} catch {
    Write-Error "API unreachable at $ApiBase"
    exit 1
}

if ([string]::IsNullOrWhiteSpace($Account) -or [string]::IsNullOrWhiteSpace($Password)) {
    Write-Error "blocked_by_real_credentials"
    Write-Host "hint: set TRADEMIND_ADMIN_ACCOUNT and TRADEMIND_ADMIN_PASSWORD"
    Exit-Blocked
}

$loginBody = @{ account = $Account; password = $Password } | ConvertTo-Json
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" -Body $loginBody
$token = $login.data.token
if (-not $token) {
    Write-Error "login failed"
    exit 1
}

$preflightBody = if ($LiveTest -eq "true" -or $LiveTest -eq "1") { '{"liveTest":true}' } else { "{}" }
$preflight = Invoke-ApiJson -Method Post -Url "$ApiV1/platform/douyin/production-preflight" -Body $preflightBody -Token $token
$latest = Invoke-ApiJson -Method Get -Url "$ApiV1/platform/douyin/production-preflight/latest" -Token $token
$runtime = Invoke-ApiJson -Method Get -Url "$ApiV1/platform/douyin/runtime-status" -Token $token

$payload = @{
    health = $health.data
    preflight = $preflight.data
    latest = $latest.data
    runtime = $runtime.data
} | ConvertTo-Json -Depth 20
Set-Content -Path $Out -Value $payload -Encoding UTF8
Write-Host "report: $Out"

if ($preflight.data.blockedByRealCredentials -eq $true) {
    Exit-Blocked
}
Write-Host "ok: preflight completed"
