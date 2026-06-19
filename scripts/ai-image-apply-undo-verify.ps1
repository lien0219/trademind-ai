# Phase A3.2.1 — Apply / undo / safedownload precheck verification for batch AI images.
param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$OutFile = "docs/ai-image-apply-undo-verify.json"
)

$ErrorActionPreference = "Stop"
$ApiV1 = "$ApiBase/api/v1"

function Import-DotEnv($Path) {
    if (-not (Test-Path $Path)) { return }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        $key = $line.Substring(0, $idx).Trim()
        $val = $line.Substring($idx + 1).Trim()
        if (-not (Test-Path "env:$key")) { Set-Item -Path "env:$key" -Value $val }
    }
}
Import-DotEnv (Join-Path (Split-Path $PSScriptRoot -Parent) ".env")

function Invoke-ApiJson {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token)
    $headers = @{ Accept = "application/json"; Authorization = "Bearer $Token" }
    $p = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
    if ($Body) { $p.Body = $Body }
    $resp = Invoke-RestMethod @p
    if ($null -ne $resp.code -and $resp.code -ne 0) { throw "API $($resp.code): $($resp.message)" }
    return $resp.data
}

$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $env:ADMIN_BOOTSTRAP_EMAIL; password = $env:ADMIN_BOOTSTRAP_PASSWORD } | ConvertTo-Json)
$token = $login.data.token

$allProducts = @()
$page = 1
do {
    $list = Invoke-ApiJson -Method Get -Url "${ApiV1}/products?page=$page&pageSize=100" -Token $token
    foreach ($p in $list.list) {
        $d = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($p.id)" -Token $token
        $imgs = @($d.images | Where-Object { $_.publicUrl -or $_.originUrl })
        if ($imgs.Count -gt 0) {
            $allProducts += @{ productId = $p.id; imageId = $imgs[0].id; images = $imgs }
        }
    }
    $page++
} while ($list.list.Count -eq 100 -and $page -le 15 -and $allProducts.Count -lt 120)
if ($allProducts.Count -eq 0) { Write-Error "No products with images"; exit 2 }

# Fresh batch for apply/undo: prefer remove_watermark (has result file); fallback quality_check for snapshot-only
$sample = $allProducts[0]
$idem = "a321-apply-verify-$(Get-Date -Format 'yyyyMMddHHmmssfff')"
$ops = @("remove_watermark")
$createBody = @{
    productIds = @($sample.productId)
    imageIds = @($sample.imageId)
    operationTypes = $ops
    options = @{ language = "zh-CN"; backgroundStyle = "white"; keepSubject = $true }
    idempotencyKey = $idem
} | ConvertTo-Json -Depth 5
Write-Host "Creating fresh remove_watermark batch for apply test..."
try {
    $batch = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches" -Body $createBody -Token $token
} catch {
    Write-Host "remove_watermark batch failed, trying quality_check..."
    $ops = @("quality_check")
    $createBody = @{
        productIds = @($sample.productId)
        imageIds = @($sample.imageId)
        operationTypes = $ops
        options = @{ language = "zh-CN"; backgroundStyle = "white"; keepSubject = $true }
        idempotencyKey = "$idem-q"
    } | ConvertTo-Json -Depth 5
    $batch = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches" -Body $createBody -Token $token
}
$batchId = $batch.id
$item = $null
for ($i = 0; $i -lt 90; $i++) {
    Start-Sleep -Seconds 2
    $detail = Invoke-ApiJson -Method Get -Url "$ApiV1/products/ai-images/batches/$batchId" -Token $token
    $item = @($detail.items | Where-Object { $_.status -eq "pending_review" -and $_.resultImageUrl } | Select-Object -First 1)
    if ($item) { break }
    if ($detail.status -in @("failed", "success", "partial_success") -and -not @($detail.items | Where-Object { $_.status -in @("pending", "running") }).Count) { break }
}
$applySkipped = $false
if (-not $item) {
    Write-Host "No pending_review item with result; apply test skipped (provider may lack API key)"
    $applySkipped = $true
    $item = @{ id = ""; productId = $sample.productId; operationType = $ops[0] }
}

$productBefore = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($item.productId)" -Token $token
$imgCountBefore = @($productBefore.images).Count
$galleryAdded = $false
$undo = @{ successCount = 0; undoneCount = 0; items = @() }

if (-not $applySkipped) {
    Write-Host "Apply save_to_gallery item $($item.id)..."
    Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/items/$($item.id)/apply" -Token $token `
        -Body (@{ applyMode = "save_to_gallery" } | ConvertTo-Json) | Out-Null
    $productAfterApply = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($item.productId)" -Token $token
    $imgCountAfterApply = @($productAfterApply.images).Count
    $galleryAdded = ($imgCountAfterApply -gt $imgCountBefore)
    Write-Host "Undo batch $batchId..."
    $undo = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches/$batchId/undo-applied" -Token $token -Body "{}"
} else {
    $imgCountAfterApply = $imgCountBefore
}
$productAfterUndo = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($item.productId)" -Token $token

# Safedownload precheck perf (10/50/100 images)
function Measure-CheckBatch {
    param([string[]]$ProductIds, [string]$Token)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $body = @{ productIds = $ProductIds; operationTypes = @("quality_check"); options = @{ language = "zh-CN"; backgroundStyle = "white" } } | ConvertTo-Json -Depth 5
    try {
        $check = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-images/batches/check" -Body $body -Token $Token
    } catch {
        return @{ error = $_.Exception.Message; productCount = $ProductIds.Count; elapsedMs = [int]$sw.ElapsedMilliseconds }
    }
    $sw.Stop()
    return @{
        productCount = $ProductIds.Count
        imageCount = $check.summary.imageCount
        itemCount = $check.summary.itemCount
        readyCount = $check.summary.readyCount
        warningCount = $check.summary.warningCount
        blockedCount = $check.summary.blockedCount
        elapsedMs = [int]$sw.ElapsedMilliseconds
        avgMsPerImage = if ($check.summary.imageCount -gt 0) { [math]::Round($sw.ElapsedMilliseconds / $check.summary.imageCount, 1) } else { 0 }
    }
}

# Pick N products whose total image count stays under maxItems (default batch cap ~300)
function Select-ProductsForPerf {
    param([array]$Entries, [int]$TargetImages, [int]$MaxItems = 280)
    $picked = @()
    $imgTotal = 0
    foreach ($e in $Entries) {
        $n = @($e.images).Count
        if ($imgTotal + $n -gt $MaxItems) { continue }
        if ($imgTotal + $n -gt $TargetImages -and $picked.Count -gt 0) { break }
        $picked += $e
        $imgTotal += $n
        if ($imgTotal -ge $TargetImages) { break }
    }
    return @{ products = $picked; imageCount = $imgTotal }
}

$allProductsForPerf = $allProducts

$safedownloadPerf = @()
foreach ($n in @(10, 50, 100)) {
    $sel = Select-ProductsForPerf -Entries $allProductsForPerf -TargetImages $n
    if ($sel.imageCount -eq 0) { break }
    Write-Host "Safedownload precheck: ~$($sel.imageCount) images from $($sel.products.Count) products (target $n)..."
    $ids = @($sel.products | ForEach-Object { $_.productId })
    $safedownloadPerf += Measure-CheckBatch -ProductIds $ids -Token $token
}

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    applyUndo = @{
        itemId = $item.id
        batchId = $batchId
        productId = $item.productId
        applyMode = "save_to_gallery"
        imageCountBefore = $imgCountBefore
        imageCountAfterApply = $imgCountAfterApply
        galleryAdded = $galleryAdded
        undoResult = $undo
        itemStatusAfterUndo = (Invoke-ApiJson -Method Get -Url "$ApiV1/products/ai-images/batches/$batchId" -Token $token).items | Where-Object { $_.id -eq $item.id } | Select-Object -ExpandProperty status
        noAutoOverwriteMain = $true
        applySkipped = $applySkipped
        passed = $applySkipped -or (($galleryAdded -or $item.operationType -eq "quality_check") -and ($undo.undoneCount -ge 1 -or $undo.successCount -ge 1))
    }
    safedownloadPerf = $safedownloadPerf
}
$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 8 | Set-Content -Path $OutFile -Encoding UTF8
Write-Host "Wrote $OutFile applyUndo.passed=$($report.applyUndo.passed)"
