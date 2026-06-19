# Phase A3.2.1 — Real Image Provider trial run for batch AI images (small sample).
# Usage: .\scripts\ai-image-trial-run.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/ai-image-trial-run.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [int]$PollSec = 5,
    [int]$MaxWaitSec = 600,
    [string]$OutFile = "docs/ai-image-trial-run.json"
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

function Invoke-ApiJson {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token = $null)
    $headers = @{ Accept = "application/json" }
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $params = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
    if ($Body) { $params.Body = $Body }
    $resp = Invoke-RestMethod @params
    if ($null -ne $resp.code -and $resp.code -ne 0) {
        throw "API error ($($resp.code)): $($resp.message)"
    }
    return $resp.data
}

function Wait-BatchDone {
    param([string]$BatchId, [string]$Token, [int]$ExpectedItems)
    $deadline = (Get-Date).AddSeconds($MaxWaitSec)
    do {
        Start-Sleep -Seconds $PollSec
        $detail = Invoke-ApiJson -Method Get -Url "$ApiV1/products/ai-images/batches/$BatchId" -Token $Token
        $pending = @($detail.items | Where-Object { $_.status -in @("pending", "running") })
        $status = $detail.status
        Write-Host "  batch $BatchId status=$status pending/running=$($pending.Count)/$ExpectedItems"
        if ($status -in @("completed", "partial_success", "failed", "success") -and $pending.Count -eq 0) {
            return $detail
        }
        $reviewReady = @($detail.items | Where-Object { $_.status -in @("pending_review", "failed", "rejected", "applied", "cancelled", "conflict") }).Count
        if ($reviewReady -ge $ExpectedItems -and $pending.Count -eq 0) {
            return $detail
        }
    } while ((Get-Date) -lt $deadline)
    throw "Batch $BatchId did not finish within ${MaxWaitSec}s"
}

function Get-ProductsWithImages {
    param([string]$Token, [int]$MinImages = 1, [int]$Limit = 30)
    $candidates = @()
    $page = 1
    do {
        $list = Invoke-ApiJson -Method Get -Url "${ApiV1}/products?page=$page&pageSize=100" -Token $Token
        if (-not $list.list) { break }
        foreach ($p in $list.list) {
            $detail = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($p.id)" -Token $Token
            $imgs = @($detail.images | Where-Object { $_.publicUrl -or $_.originUrl })
            if ($imgs.Count -ge $MinImages) {
                $candidates += @{
                    productId = $p.id
                    title = $p.title
                    images = $imgs
                }
            }
            if ($candidates.Count -ge $Limit) { break }
        }
        if ($candidates.Count -ge $Limit) { break }
        $page++
    } while ($list.list.Count -eq 100 -and $page -le 20)
    return $candidates
}

function Get-ProviderStatus {
    param([string]$Token)
    $overview = Invoke-ApiJson -Method Get -Url "$ApiV1/settings/integrations/overview" -Token $Token
    $img = $overview.image
    $provider = if ($img.providerCurrent) { $img.providerCurrent } else { "noop" }
    $configured = ($provider -ne "" -and $provider -ne "noop")
    $hasKey = $false
    if ($img.removeBg) { $hasKey = $true }
    if ($img.openAIImage) { $hasKey = $true }
    if ($img.comfyUI) { $hasKey = $true }
    # Also check settings group for dashscope/volcengine/siliconflow keys
    $settings = Invoke-ApiJson -Method Get -Url "$ApiV1/settings" -Token $Token
    $imgRows = @($settings | Where-Object { $_.groupKey -eq "image" })
    $keyFields = @("dashscope_image_api_key", "volcengine_image_api_key", "siliconflow_image_api_key", "removebg_api_key", "openai_image_api_key", "comfyui_api_key")
    foreach ($kf in $keyFields) {
        $row = $imgRows | Where-Object { $_.itemKey -eq $kf } | Select-Object -First 1
        if ($row -and $row.itemValue -and $row.itemValue -ne "") { $hasKey = $true }
    }
    return @{
        provider = $provider
        configured = $configured
        hasApiKey = $hasKey
        overview = $img
    }
}

if (-not $Account -or -not $Password) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD in .env"
    exit 1
}

Write-Host "Logging in..."
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

Write-Host "Checking image Provider config..."
$provStatus = Get-ProviderStatus -Token $token
Write-Host "  provider=$($provStatus.provider) configured=$($provStatus.configured) hasKey=$($provStatus.hasApiKey)"

if (-not $provStatus.configured) {
    $report = @{
        generatedAt = (Get-Date).ToUniversalTime().ToString("o")
        apiBase = $ApiBase
        conclusion = "blocked_by_image_provider"
        providerStatus = $provStatus
        message = "Image provider not configured or noop. Configure in Settings > Image Processing."
        batches = @()
        items = @()
    }
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $report | ConvertTo-Json -Depth 10 | Set-Content -Path $OutFile -Encoding UTF8
    Write-Host "BLOCKED: image provider not configured"
    exit 3
}

Write-Host "Discovering products with images..."
$candidates = Get-ProductsWithImages -Token $token -MinImages 1 -Limit 20
if ($candidates.Count -lt 3) {
    Write-Error "Need at least 3 products with images; found $($candidates.Count)"
    exit 2
}
Write-Host "  found $($candidates.Count) products with images"

$defaultOpts = @{
    language = "zh-CN"
    backgroundStyle = "white"
    keepSubject = $true
    keepBrandLogo = $false
    skipFailedImages = $true
    outputFormat = "png"
}

function New-TrialBatch {
    param([string]$Label, [array]$ProductEntries, [string[]]$Ops, [int]$ImagesPerProduct = 1)
    $productIds = @($ProductEntries | ForEach-Object { $_.productId })
    $imageIds = @()
    foreach ($entry in $ProductEntries) {
        $picked = @($entry.images | Where-Object { $_.imageType -eq "main" } | Select-Object -First $ImagesPerProduct)
        if ($picked.Count -lt $ImagesPerProduct) {
            $picked = @($entry.images | Select-Object -First $ImagesPerProduct)
        }
        foreach ($img in $picked) { $imageIds += $img.id }
    }
    if ($imageIds.Count -eq 0) {
        throw "No image IDs for batch $Label"
    }
    $idem = "a321-trial-$Label-$(Get-Date -Format 'yyyyMMddHHmmssfff')"
    $body = @{
        productIds = $productIds
        imageIds = $imageIds
        operationTypes = $Ops
        options = $defaultOpts
        idempotencyKey = $idem
    } | ConvertTo-Json -Depth 6
    Write-Host "Creating batch $Label (products=$($productIds.Count) images=$($imageIds.Count) ops=$($Ops -join ','))..."
    $created = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches" -Body $body -Token $token
    $expectedItems = $imageIds.Count * $Ops.Count
    $detail = Wait-BatchDone -BatchId $created.id -Token $token -ExpectedItems $expectedItems
    return @{ label = $Label; batch = $detail; expectedItems = $expectedItems; ops = $Ops; imageIds = $imageIds }
}

# Build sample sets: 5 quality, 5 white bg, 3 watermark, 3 select best main
$qualityProducts = @($candidates | Select-Object -First 5)
$whiteBgProducts = @($candidates | Select-Object -Skip 5 -First 5)
$watermarkProducts = @($candidates | Select-Object -Skip 10 -First 3)
$selectMainProducts = @($candidates | Select-Object -Skip 13 -First 3)

$batches = @()
$checkResults = @()

try {
    # Pre-flight check for quality batch
    $qIds = @($qualityProducts | ForEach-Object { $_.productId })
    $checkBody = @{
        productIds = $qIds
        operationTypes = @("quality_check")
        options = $defaultOpts
    } | ConvertTo-Json -Depth 6
    $check = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches/check" -Body $checkBody -Token $token
    Write-Host "Pre-flight: ready=$($check.summary.readyCount) warning=$($check.summary.warningCount) blocked=$($check.summary.blockedCount)"
    $checkResults += @{ label = "quality_check"; summary = $check.summary }

    $batches += New-TrialBatch -Label "I1-quality" -ProductEntries $qualityProducts -Ops @("quality_check")
    $batches += New-TrialBatch -Label "I2-white-bg" -ProductEntries $whiteBgProducts -Ops @("white_background")
    $batches += New-TrialBatch -Label "I3-watermark" -ProductEntries $watermarkProducts -Ops @("remove_watermark")
    $batches += New-TrialBatch -Label "I4-select-main" -ProductEntries $selectMainProducts -Ops @("select_best_main")
} catch {
    $errMsg = $_.Exception.Message
    $report = @{
        generatedAt = (Get-Date).ToUniversalTime().ToString("o")
        conclusion = "blocked_by_image_provider"
        error = $errMsg
        providerStatus = $provStatus
        batches = $batches
        checkResults = $checkResults
    }
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $report | ConvertTo-Json -Depth 12 | Set-Content -Path $OutFile -Encoding UTF8
    Write-Error $errMsg
    exit 3
}

$itemResults = @()
$failedItems = 0
$emptyResults = 0
$pendingReview = 0
$qualityWarnings = 0
$autoOverwrite = 0

foreach ($b in $batches) {
    foreach ($it in $b.batch.items) {
        $resultURL = if ($it.resultImageUrl) { $it.resultImageUrl.Trim() } else { "" }
        $hasWarning = ($it.qualityWarnings -and $it.qualityWarnings.Count -gt 0)
        if ($hasWarning) { $qualityWarnings++ }
        if ($it.status -eq "failed") { $failedItems++ }
        if ($it.status -eq "pending_review" -and $resultURL.Length -eq 0 -and $it.operationType -ne "quality_check") { $emptyResults++ }
        if ($it.status -eq "pending_review") { $pendingReview++ }

        $prod = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($it.productId)" -Token $token
        $origMainCount = @($prod.images | Where-Object { $_.isBestMain }).Count
        $overwritten = $false
        if ($it.status -eq "applied") { $overwritten = $true; $autoOverwrite++ }

        $itemResults += @{
            batchLabel = $b.label
            batchId = $b.batch.id
            itemId = $it.id
            productId = $it.productId
            imageId = $it.imageId
            imageType = $it.imageType
            operationType = $it.operationType
            status = $it.status
            sourceImageUrl = $it.sourceImageUrl
            resultImageUrl = $resultURL
            hasResult = ($resultURL.Length -gt 0 -or $it.operationType -eq "quality_check")
            qualityWarningCount = if ($it.qualityWarnings) { $it.qualityWarnings.Count } else { 0 }
            qualityWarnings = $it.qualityWarnings
            errorMessage = $it.errorMessage
            autoOverwrite = $overwritten
            productMainCount = $origMainCount
        }
    }
}

Write-Host "Checking taskcenter ai_image failures..."
$tcFailures = Invoke-ApiJson -Method Get -Url "${ApiV1}/task-center/failures?taskType=ai_image&page=1&pageSize=50" -Token $token
$tcCount = if ($tcFailures.list) { $tcFailures.list.Count } else { 0 }

$totalItems = $itemResults.Count
$successItems = @($itemResults | Where-Object { $_.status -eq "pending_review" -and ($_.hasResult -or $_.operationType -eq "quality_check") }).Count
$conclusion = "passed"
if ($failedItems -gt 0 -or $emptyResults -gt 0) {
    $conclusion = if ($failedItems -eq $totalItems) { "blocked_by_image_provider" } else { "passed_with_warning" }
}
if ($autoOverwrite -gt 0) { $conclusion = "failed" }

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase = $ApiBase
    conclusion = $conclusion
    providerStatus = $provStatus
    totalItems = $totalItems
    successItems = $successItems
    failedItems = $failedItems
    emptyResults = $emptyResults
    pendingReview = $pendingReview
    qualityWarningItems = $qualityWarnings
    taskCenterAiImageFailures = $tcCount
    checkResults = $checkResults
    batches = @($batches | ForEach-Object {
        @{
            label = $_.label
            batchId = $_.batch.id
            batchNo = $_.batch.batchNo
            status = $_.batch.status
            successCount = $_.batch.successCount
            failedCount = $_.batch.failedCount
            operationTypes = $_.ops
        }
    })
    items = $itemResults
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 12 | Set-Content -Path $OutFile -Encoding UTF8

Write-Host ""
Write-Host "Trial complete: $conclusion - success=$successItems total=$totalItems failed=$failedItems"
Write-Host "Wrote $OutFile"

if ($conclusion -eq "failed") { exit 4 }
if ($conclusion -eq "blocked_by_image_provider") { exit 3 }
exit 0
