# Douyin E2E write chain — requires ALLOW_DOUYIN_WRITE_TEST=true and real credentials.
$ErrorActionPreference = "Stop"

$ApiBase = if ($env:TRADEMIND_API_BASE) { $env:TRADEMIND_API_BASE.TrimEnd("/") } else { "http://127.0.0.1:8080" }
$ApiV1 = "$ApiBase/api/v1"
$Account = $env:TRADEMIND_ADMIN_ACCOUNT
$Password = $env:TRADEMIND_ADMIN_PASSWORD
$AllowWrite = $env:ALLOW_DOUYIN_WRITE_TEST
$ProductId = $env:DOUYIN_E2E_PRODUCT_ID
$ShopId = $env:DOUYIN_E2E_SHOP_ID
$ReportDir = if ($env:DOUYIN_E2E_REPORT_DIR) { $env:DOUYIN_E2E_REPORT_DIR } else { "./tmp/douyin-e2e" }

New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null
$Ts = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$Out = Join-Path $ReportDir "write-$Ts.json"

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
if ($AllowWrite -ne "true" -and $AllowWrite -ne "1") {
    Write-Error "set ALLOW_DOUYIN_WRITE_TEST=true to run write probes"
    exit 4
}

$loginBody = @{ account = $Account; password = $Password } | ConvertTo-Json
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" -Body $loginBody
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

$preflight = Invoke-ApiJson -Method Post -Url "$ApiV1/platform/douyin/production-preflight" -Body '{"liveTest":true}' -Token $token
if ($preflight.data.blockedByRealCredentials -eq $true) { Exit-Blocked }

$writeProbes = @{ skipped = @("create-draft", "sync-inventory"); reason = "set DOUYIN_E2E_PRODUCT_ID and DOUYIN_E2E_SHOP_ID for write probes" }
if (-not [string]::IsNullOrWhiteSpace($ProductId) -and -not [string]::IsNullOrWhiteSpace($ShopId)) {
    $validate = Invoke-ApiJson -Method Post -Url "$ApiV1/products/$ProductId/platform-configs/douyin_shop/validate" -Body "{}" -Token $token
    $imgStatus = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$ProductId/platform-configs/douyin_shop/images/status" -Token $token
    $writeProbes = @{
        validate = $validate.data
        imageStatus = $imgStatus.data
        note = "manual create-draft/sync-inventory via admin when ready"
    }
}

$payload = @{ preflight = $preflight.data; writeProbes = $writeProbes } | ConvertTo-Json -Depth 20
Set-Content -Path $Out -Value $payload -Encoding UTF8
Write-Host "report: $Out"
Write-Host "ok: write E2E scaffold completed"
