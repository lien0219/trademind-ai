# Phase A3.1.2 — Real AI Provider trial run for batch AI text (13 items across 3 batches).
# Usage: .\scripts\ai-text-trial-run.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/ai-text-trial-run.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [int]$PollSec = 5,
    [int]$MaxWaitSec = 600,
    [string]$OutFile = "docs/ai-text-trial-run.json"
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
        $detail = Invoke-ApiJson -Method Get -Url "$ApiV1/products/ai-text/batches/$BatchId" -Token $Token
        $pending = @($detail.items | Where-Object { $_.status -in @("pending", "running") })
        $status = $detail.status
        Write-Host "  batch $BatchId status=$status pending/running=$($pending.Count)/$ExpectedItems"
        if ($pending.Count -gt 0 -and $status -eq "running") {
            $stuckSec = ((Get-Date) - [datetime]$detail.createdAt).TotalSeconds
            if ($stuckSec -gt 90) {
                Write-Host "  retrying stuck items (pending/running)..."
                try {
                    Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-text/batches/$BatchId/retry-failed" -Token $Token | Out-Null
                } catch {
                    foreach ($it in $pending) {
                        Write-Host "  regenerate item $($it.id)..."
                        Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-text/items/$($it.id)/regenerate" -Token $Token | Out-Null
                    }
                }
            }
        }
        if ($status -in @("completed", "partial_success", "failed", "success") -and $pending.Count -eq 0) {
            return $detail
        }
        $reviewReady = @($detail.items | Where-Object { $_.status -in @("pending_review", "failed", "rejected", "applied", "cancelled") }).Count
        if ($reviewReady -ge $ExpectedItems -and $pending.Count -eq 0) {
            return $detail
        }
    } while ((Get-Date) -lt $deadline)
    throw "Batch $BatchId did not finish within ${MaxWaitSec}s"
}

if (-not $Account -or -not $Password) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD"
    exit 1
}

# Sample IDs from docs/BATCH_AI_TEXT_UX_ACCEPTANCE.md
$titleOnly = @(
    "715b12b8-c23a-42a3-bcc9-f2dd69d47095",
    "4fe45e34-4529-438b-a64d-a478b412118c",
    "ac15338f-2e45-40c3-89bc-8c9150fa49b7",
    "a7715a98-3291-4bc3-ab7d-6620b07371af",
    "8dfe5af3-554a-4110-9af8-ad1f2165583b"
)
$descOnly = @(
    "93b9663d-a5f4-4810-a9bc-2e3dcaf9f87d",
    "4ee03ff7-4239-4d10-ae6b-ba72ffb468aa",
    "e1f60994-f434-47ef-b8fe-d5593d6b8118",
    "28aa935b-03a5-440b-8697-51cd3f95de02",
    "077fb62d-f936-4b13-96e3-15014dfa3f58"
)
$both = @(
    "1cf3566d-9aaf-44ed-b8eb-293ec5d16031",
    "249a2bac-a1d4-43c8-bbff-7b909c100ab9",
    "1ae4dd72-541f-43d7-8051-05dd32adf984"
)

$defaultOpts = @{
    language = "zh-CN"
    platform = "douyin_shop"
    tone = "professional"
    maxLength = 60
    titleStyle = "seo"
    highlightSelling = $true
    removeCollectNoise = $true
    descStyle = "structured"
    descStructure = "bullet"
    highlightScenarios = $true
    generateBullets = $true
}

Write-Host "Logging in..."
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

Write-Host "Pre-flight check..."
$checkBody = @{
    productIds = ($titleOnly + $descOnly + $both | Select-Object -Unique)
    operationTypes = @("title", "description")
    options = $defaultOpts
} | ConvertTo-Json -Depth 6
try {
    $check = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-text/batches/check" -Body $checkBody -Token $token
    Write-Host "  ready=$($check.summary.readyCount) warning=$($check.summary.warningCount) blocked=$($check.summary.blockedCount)"
} catch {
    Write-Error "Pre-flight check failed: $_"
    exit 2
}

$batches = @()
$allItems = @()

function New-TrialBatch {
    param([string]$Label, [string[]]$ProductIds, [string[]]$Ops)
    $idem = "a312-trial-$Label-$(Get-Date -Format 'yyyyMMddHHmmss')"
    $body = @{
        productIds = $ProductIds
        operationTypes = $Ops
        options = $defaultOpts
        idempotencyKey = $idem
    } | ConvertTo-Json -Depth 6
    Write-Host "Creating batch $Label ($($ProductIds.Count) products, ops=$($Ops -join ','))..."
    $created = Invoke-ApiJson -Method Post -Url "$ApiV1/products/ai-text/batches" -Body $body -Token $token
    $expectedItems = $ProductIds.Count * $Ops.Count
    $detail = Wait-BatchDone -BatchId $created.id -Token $token -ExpectedItems $expectedItems
    return @{ label = $Label; batch = $detail; expectedItems = $expectedItems }
}

try {
    $batches += New-TrialBatch -Label "T1-title" -ProductIds $titleOnly -Ops @("title")
    $batches += New-TrialBatch -Label "T2-description" -ProductIds $descOnly -Ops @("description")
    $batches += New-TrialBatch -Label "T3-both" -ProductIds $both -Ops @("title", "description")
} catch {
    $errMsg = $_.Exception.Message
    $report = @{
        generatedAt = (Get-Date).ToUniversalTime().ToString("o")
        conclusion = "blocked_by_ai_provider"
        error = $errMsg
        batches = $batches
    }
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $report | ConvertTo-Json -Depth 10 | Set-Content -Path $OutFile -Encoding UTF8
    Write-Error $errMsg
    exit 3
}

$itemResults = @()
$failedItems = 0
$emptyItems = 0
$pendingReview = 0
$qualityWarnings = 0

foreach ($b in $batches) {
    foreach ($it in $b.batch.items) {
        $gen = if ($it.generatedText) { $it.generatedText.Trim() } else { "" }
        $hasWarning = ($it.qualityWarnings -and $it.qualityWarnings.Count -gt 0)
        if ($hasWarning) { $qualityWarnings++ }
        if ($it.status -eq "failed") { $failedItems++ }
        if ($it.status -eq "pending_review" -and $gen.Length -gt 0) { $pendingReview++ }
        if ($it.status -eq "pending_review" -and $gen.Length -eq 0) { $emptyItems++ }

        # Verify product not auto-overwritten
        $prod = Invoke-ApiJson -Method Get -Url "$ApiV1/products/$($it.productId)" -Token $token
        $autoOverwrite = $false
        if ($it.operationType -eq "title" -and $prod.title -eq $gen -and $gen.Length -gt 0) {
            $autoOverwrite = $true
        }
        if ($it.operationType -eq "description" -and $prod.description -eq $gen -and $gen.Length -gt 0) {
            $autoOverwrite = $true
        }

        $itemResults += @{
            batchLabel = $b.label
            batchId = $b.batch.id
            itemId = $it.id
            productId = $it.productId
            operationType = $it.operationType
            status = $it.status
            generatedLen = $gen.Length
            generatedPreview = if ($gen.Length -gt 80) { $gen.Substring(0, 80) + "..." } else { $gen }
            qualityWarningCount = if ($it.qualityWarnings) { $it.qualityWarnings.Count } else { 0 }
            qualityWarnings = $it.qualityWarnings
            errorMessage = $it.errorMessage
            autoOverwrite = $autoOverwrite
            productTitleUnchanged = ($prod.title -eq $it.currentContent -or $it.operationType -ne "title")
        }
    }
}

Write-Host "Checking taskcenter failures..."
$tcFailures = Invoke-ApiJson -Method Get -Url "$ApiV1/task-center/failures?taskType=ai_text&page=1&pageSize=50" -Token $token
$tcCount = if ($tcFailures.list) { $tcFailures.list.Count } else { 0 }

$totalItems = $itemResults.Count
$successItems = @($itemResults | Where-Object { $_.status -eq "pending_review" -and $_.generatedLen -gt 0 }).Count
$conclusion = "passed"
if ($failedItems -gt 0 -or $emptyItems -gt 0) {
    $conclusion = if ($failedItems -eq $totalItems) { "blocked_by_ai_provider" } else { "passed_with_warning" }
}
if (@($itemResults | Where-Object { $_.autoOverwrite }).Count -gt 0) {
    $conclusion = "failed"
}

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase = $ApiBase
    conclusion = $conclusion
    totalItems = $totalItems
    successItems = $successItems
    failedItems = $failedItems
    emptyGenerated = $emptyItems
    pendingReview = $pendingReview
    qualityWarningItems = $qualityWarnings
    taskCenterAiTextFailures = $tcCount
    batches = @($batches | ForEach-Object {
        @{
            label = $_.label
            batchId = $_.batch.id
            batchNo = $_.batch.batchNo
            status = $_.batch.status
            successCount = $_.batch.successCount
            failedCount = $_.batch.failedCount
        }
    })
    items = $itemResults
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 12 | Set-Content -Path $OutFile -Encoding UTF8

Write-Host ""
Write-Host "Trial complete: $conclusion ($successItems/$totalItems success)"
Write-Host "Wrote $OutFile"

if ($conclusion -eq "failed") { exit 4 }
if ($conclusion -eq "blocked_by_ai_provider") { exit 3 }
exit 0
