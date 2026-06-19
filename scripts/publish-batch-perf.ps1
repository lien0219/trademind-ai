# Phase A2.1 batch publish performance benchmark.
# Usage: .\scripts\publish-batch-perf.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/publish-batch-perf.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/publish-batch-perf.json"
)

$ErrorActionPreference = "Stop"
$ApiV1 = "$ApiBase/api/v1"

function Invoke-ApiJson {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token = $null)
    $headers = @{ Accept = "application/json" }
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $params = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
    if ($Body) { $params.Body = $Body }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $resp = Invoke-RestMethod @params
    $sw.Stop()
    if ($null -ne $resp.code -and $resp.code -ne 0) {
        throw "API error: $($resp.message)"
    }
    return @{ data = $resp.data; ms = [math]::Round($sw.Elapsed.TotalMilliseconds, 1) }
}

function Measure-BatchScenario {
    param(
        [string]$Token,
        [string]$Name,
        [int]$ProductCount,
        [int]$TargetCount
    )
    Write-Host "Scenario $Name (${ProductCount}x${TargetCount})..."

    $allProducts = @()
    $page = 1
    do {
        $list = (Invoke-ApiJson -Method Get -Url "$ApiV1/products?page=$page&pageSize=100&status=draft" -Token $Token).data
        if ($list.list) { $allProducts += $list.list }
        $page++
    } while ($list.list -and $list.list.Count -eq 100 -and $page -le 30)

    if ($allProducts.Count -lt $ProductCount) {
        throw "Need at least $ProductCount draft products, found $($allProducts.Count)"
    }
    $productIds = @($allProducts | Select-Object -First $ProductCount | ForEach-Object { $_.id })

    $targetsResp = (Invoke-ApiJson -Method Get -Url "$ApiV1/product-publish/targets" -Token $Token).data
    $shops = @()
    foreach ($p in ($targetsResp.platforms | Where-Object { $_.capability -eq 'local_draft_only' })) {
        foreach ($s in ($p.shops | Where-Object { $_.shopId })) {
            $shops += @{ platform = $p.platform; shopId = $s.shopId }
            if ($shops.Count -ge $TargetCount) { break }
        }
        if ($shops.Count -ge $TargetCount) { break }
    }
    if ($shops.Count -lt $TargetCount) {
        throw "Need $TargetCount local_draft_only targets, found $($shops.Count)"
    }
    $targets = @($shops | Select-Object -First $TargetCount)

    $createBody = @{
        productIds      = $productIds
        targets         = $targets
        commonConfig    = @{ remark = "perf-$Name" }
        overrides       = @{}
        includeWarnings = $true
        name            = "perf $Name"
    } | ConvertTo-Json -Depth 8 -Compress

    $checkBody = @{
        productIds   = $productIds
        targets      = $targets
        commonConfig = @{ remark = "perf-$Name" }
        overrides    = @{}
    } | ConvertTo-Json -Depth 8 -Compress

    $check = Invoke-ApiJson -Method Post -Url "$ApiV1/product-publish/batch-targets/check" -Body $checkBody -Token $Token
    $create = Invoke-ApiJson -Method Post -Url "$ApiV1/product-publish/batch-targets/create-drafts" -Body $createBody -Token $Token

    $batchId = $create.data.batchId
    $detail = Invoke-ApiJson -Method Get -Url "$ApiV1/product-publish/batches/$batchId" -Token $Token

    return @{
        name              = $Name
        productCount      = $ProductCount
        targetCount       = $TargetCount
        expectedTasks     = $ProductCount * $TargetCount
        checkMs           = $check.ms
        createMs          = $create.ms
        detailMs          = $detail.ms
        batchId           = $batchId
        readyCount        = $check.data.summary.readyCount
        warningCount      = $check.data.summary.warningCount
        blockedCount      = $check.data.summary.blockedCount
        taskCount         = $create.data.taskCount
        successCount      = $create.data.successCount
        failedCount       = $create.data.failedCount
        skippedCount      = $create.data.skippedCount
        externalApiCalled = $false
        aiTasksCreated    = $false
        sqlQueryCount     = $null
        note              = "local_draft_only targets only; SQL count not instrumented in MVP"
    }
}

if (-not $Account -or -not $Password) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD"
    exit 1
}

Write-Host "Logging in..."
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

$scenarios = @(
    @{ Name = "20x2"; Products = 20; Targets = 2 },
    @{ Name = "50x2"; Products = 50; Targets = 2 },
    @{ Name = "100x3"; Products = 100; Targets = 3 }
)

$results = @()
foreach ($s in $scenarios) {
    $results += Measure-BatchScenario -Token $token -Name $s.Name -ProductCount $s.Products -TargetCount $s.Targets
}

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase     = $ApiBase
    scenarios   = $results
}
$json = $report | ConvertTo-Json -Depth 8
Set-Content -Path $OutFile -Value $json -Encoding UTF8
Write-Host "Wrote $OutFile"
$results | Format-Table name, checkMs, createMs, detailMs, taskCount, successCount, failedCount
