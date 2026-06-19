# Phase A1.1 acceptance automation: login, sample inventory, progress/readiness checks.
# Usage: .\scripts\a1-acceptance-run.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/a1-acceptance-run.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/a1-acceptance-run.json"
)

$ErrorActionPreference = "Stop"
$ApiV1 = "$ApiBase/api/v1"

function Invoke-ApiJson {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token = $null)
    $headers = @{ Accept = "application/json" }
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $params = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
    if ($Body) { $params.Body = $Body }
    $resp = Invoke-RestMethod @params
    if ($null -ne $resp.code -and $resp.code -ne 0) {
        throw "API error: $($resp.message)"
    }
    return $resp.data
}

if (-not $Account -or -not $Password) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD or pass -Account/-Password"
    exit 1
}

Write-Host "Logging in..."
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

$failedCodes = @(
    "product.title_missing", "product.description_missing", "product.currency_missing", "product.archived",
    "sku.none", "pricing.price_missing", "pricing.price_invalid", "image.main_missing", "image.asset_missing",
    "image.detail_missing", "inventory.stock_zero", "platform.shop_required", "platform.shop_not_authorized",
    "platform.publish_config_incomplete", "platform.publish_field_missing", "DOUYIN_SHOP_NOT_AUTHORIZED",
    "CATEGORY_REQUIRED", "PLATFORM_ATTRIBUTES_REQUIRED", "collect.taobao_tmall.price_missing"
)

function Get-ReadinessAction {
    param([string]$Code)
    $c = if ($Code) { $Code.Trim().ToLower() } else { "" }
    if (-not $c) { return $null }
    if ($c -eq "product.ai_title_missing") { return @{ tab = "ai" } }
    if ($c.StartsWith("product.title")) { return @{ tab = "basic"; section = "title" } }
    if ($c.StartsWith("product.description")) { return @{ tab = "basic"; section = "description" } }
    if ($c -eq "product.currency_missing" -or $c -eq "product.archived") { return @{ tab = "basic"; section = "title" } }
    if ($c.StartsWith("collect.") -or $c.EndsWith(".attributes_missing") -or $c.EndsWith(".stock_unknown")) {
        return @{ tab = "basic"; section = "collect-review" }
    }
    if ($c.StartsWith("image.")) { return @{ tab = "images"; section = "image-list" } }
    if ($c.StartsWith("pricing.") -or $c.StartsWith("sku.")) { return @{ tab = "skus"; section = "pricing" } }
    if ($c.StartsWith("inventory.")) { return @{ tab = "inventory"; section = "local-skus" } }
    if ($c -match "^category_required$|^platform_attributes_required$|^publish_config_missing$") {
        return @{ tab = "publish"; section = "publish-config" }
    }
    if ($c -match "shop_not_authorized|shop_token_missing|douyin_shop_not_authorized|shop_required|shop_not_found|shop_inactive") {
        return @{ href = "/shops" }
    }
    if ($c.StartsWith("platform.") -or $c.StartsWith("douyin_")) {
        return @{ tab = "publish"; section = "publish-config" }
    }
    return $null
}

Write-Host "Fetching products..."
$allProducts = @()
$page = 1
do {
    $url = "$ApiV1/products?page=$page" + "&pageSize=100"
    $list = Invoke-ApiJson -Method Get -Url $url -Token $token
    if ($list.list) { $allProducts += $list.list }
    $page++
} while ($list.list -and $list.list.Count -eq 100 -and $page -le 20)

Write-Host "Total products: $($allProducts.Count)"

$samples = @()
$actionCoverage = @{}
foreach ($code in $failedCodes) {
    $actionCoverage[$code] = [bool](Get-ReadinessAction -Code $code)
}

$sourceCounts = @{}
foreach ($p in $allProducts) {
    $src = if ($p.source) { $p.source.ToLower() } else { "unknown" }
    if (-not $sourceCounts.ContainsKey($src)) { $sourceCounts[$src] = 0 }
    $sourceCounts[$src]++
}

foreach ($p in $allProducts) {
    $id = $p.id
    $progress = $null
    $readiness = $null
    $targets = $null
    try {
        $progress = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$id/operation-progress" -Token $token
    } catch { $progress = @{ error = $_.Exception.Message } }
    try {
        $readUrl = "$ApiV1/products/$id/readiness?platform=douyin_shop" + "&mode=publish"
        $readiness = Invoke-ApiJson -Method Get -Url $readUrl -Token $token
    } catch { $readiness = @{ error = $_.Exception.Message } }
    try {
        $targets = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$id/publish-targets" -Token $token
    } catch { $targets = @{ error = $_.Exception.Message } }

    $failedChecks = @()
    if ($readiness.checks) {
        foreach ($c in $readiness.checks) {
            $lvl = if ($c.level) { $c.level.ToLower() } else { "" }
            if ($lvl -in @("failed", "error")) {
                $fx = Get-ReadinessAction -Code $c.code
                $failedChecks += @{
                    code = $c.code
                    title = $c.title
                    message = $c.message
                    hasAction = [bool]$fx
                }
            }
        }
    }

    $samples += @{
        id = $id
        source = $p.source
        title = $p.title
        skuCount = if ($p.skus) { @($p.skus).Count } else { 0 }
        operationStep = $progress.currentStep
        completionPercent = $progress.completionPercent
        publishReady = $progress.publishReady
        blockerCount = if ($progress.blockers) { @($progress.blockers).Count } else { 0 }
        warningCount = if ($progress.warnings) { @($progress.warnings).Count } else { 0 }
        failedCheckCount = $failedChecks.Count
        failedChecksWithoutAction = @($failedChecks | Where-Object { -not $_.hasAction })
        publishTargetPlatforms = if ($targets.platforms) { @($targets.platforms | ForEach-Object { $_.platform }) } else { @() }
    }
}

$missingActions = @()
foreach ($s in $samples) {
    foreach ($fc in $s.failedChecksWithoutAction) {
        $missingActions += @{ productId = $s.id; code = $fc.code }
    }
}

$perfStart = Get-Date
$listUrl = "$ApiV1/products?page=1" + "&pageSize=50"
Invoke-ApiJson -Method Get -Url $listUrl -Token $token | Out-Null
$listMs = ((Get-Date) - $perfStart).TotalMilliseconds

$result = @{
    generatedAt = (Get-Date).ToString("o")
    productCount = $allProducts.Count
    sourceCounts = $sourceCounts
    staticActionCoverage = $actionCoverage
    missingFailedActions = $missingActions
    listApiMs_page50 = [math]::Round($listMs, 1)
    samples = $samples
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$result | ConvertTo-Json -Depth 8 | Set-Content -Path $OutFile -Encoding UTF8

Write-Host "Wrote $OutFile"
Write-Host "List API (50 items): $([math]::Round($listMs, 1)) ms"
Write-Host "Failed checks without action: $($missingActions.Count)"
if ($missingActions.Count -gt 0) {
    $missingActions | Select-Object -First 10 | Format-Table
    exit 2
}
exit 0
