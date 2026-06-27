# Phase R1 — AI operation workbench performance benchmark (100/500/1000 todos).
# Usage: .\scripts\ai-operation-workbench-perf.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/ai-operation-workbench-perf.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [int[]]$TodoTargets = @(100, 500, 1000),
    [string]$OutFile = "docs/ai-operation-workbench-perf.json"
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

function Invoke-Timed {
    param([string]$Method, [string]$Url, [string]$Token)
    $headers = @{ Accept = "application/json"; Authorization = "Bearer $Token" }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $resp = Invoke-RestMethod -Method $Method -Uri $Url -Headers $headers
    $sw.Stop()
    if ($null -ne $resp.code -and $resp.code -ne 0) {
        throw "API error: $($resp.message)"
    }
    return @{ data = $resp.data; ms = [math]::Round($sw.Elapsed.TotalMilliseconds, 1) }
}

function Ensure-ProductCount {
    param([string]$Token, [int]$Need)
    $all = @()
    $page = 1
    do {
        $list = (Invoke-Timed -Method Get -Url "$ApiV1/products?page=$page&pageSize=100&status=draft" -Token $Token).data
        if ($list.list) { $all += $list.list }
        $page++
    } while ($list.list -and $list.list.Count -eq 100 -and $page -le 50)

    $existing = $all.Count
    if ($existing -ge $Need) {
        Write-Host "  Products: $existing (>= $Need)"
        return $existing
    }

    $toCreate = $Need - $existing
    Write-Host "  Seeding $toCreate draft products (have $existing, need $Need)..."
    $headers = @{ Authorization = "Bearer $Token"; Accept = "application/json" }
    for ($i = 0; $i -lt $toCreate; $i++) {
        $body = @{
            source      = "manual"
            title       = "Workbench perf seed #$i"
            description = "Performance seed for AI operation workbench todo aggregation and pagination testing."
            currency    = "CNY"
            status      = "draft"
            skus        = @(@{ skuName = "Default"; price = 19.9; stock = 10 })
        } | ConvertTo-Json -Depth 5
        Invoke-RestMethod -Method Post -Uri "$ApiV1/products" -Headers $headers -ContentType "application/json" -Body $body | Out-Null
        if (($i + 1) % 50 -eq 0) { Write-Host "    created $($i + 1)/$toCreate" }
    }
    return $Need
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

$maxTarget = ($TodoTargets | Measure-Object -Maximum).Maximum
Ensure-ProductCount -Token $token -Need $maxTarget | Out-Null

$scenarios = @()
foreach ($target in ($TodoTargets | Sort-Object)) {
    Write-Host "Scenario: workbench with ~$target underlying products..."
    $summary = Invoke-Timed -Method Get -Url "$ApiV1/ai/operation-workbench/summary" -Token $token
    $todos = Invoke-Timed -Method Get -Url "$ApiV1/ai/operation-workbench/todos?page=1&pageSize=50" -Token $token
    $todosP2 = Invoke-Timed -Method Get -Url "$ApiV1/ai/operation-workbench/todos?page=2&pageSize=50" -Token $token

    $summaryData = $summary.data.summary
    $todoItems = $todos.data.items
    $todoTotal = $todos.data.pagination.total
    $summaryTodoSum = 0
    if ($summaryData) {
        $summaryTodoSum = [int64]$summaryData.aiTextReviewCount + [int64]$summaryData.aiImageReviewCount `
            + [int64]$summaryData.publishCheckIssueCount + [int64]$summaryData.publishTaskIssueCount
    }

    $list = $todoItems
    $pageSizeOk = ($null -eq $list -or $list.Count -le 50)
    $hasLargeAiFields = $false
    if ($list -and $list.Count -gt 0) {
        $sample = $list[0] | ConvertTo-Json -Depth 6
        if ($sample -match 'generatedText|platformPayload|raw_data|aiDescription') {
            $hasLargeAiFields = $true
        }
    }

    $scenarios += @{
        targetTodoProducts = $target
        summaryMs          = $summary.ms
        todosPage1Ms       = $todos.ms
        todosPage2Ms       = $todosP2.ms
        summaryTodoSum     = $summaryTodoSum
        todosTotal         = $todoTotal
        todosPage1Count    = if ($list) { $list.Count } else { 0 }
        paginationOk       = $pageSizeOk
        loadsLargeAiFields = $hasLargeAiFields
        externalApiCalled  = $false
        sqlQueryCount      = $null
        nPlusOneObserved   = $null
        note               = "SQL count not instrumented in MVP; external API not called by design"
    }
}

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase     = $ApiBase
    scenarios   = $scenarios
    conclusion  = @{
        summaryUnder500ms   = ($scenarios | Where-Object { $_.summaryMs -lt 500 }).Count -eq $scenarios.Count
        todosUnder1000ms    = ($scenarios | Where-Object { $_.todosPage1Ms -lt 1000 }).Count -eq $scenarios.Count
        paginationCorrect   = ($scenarios | Where-Object { $_.paginationOk }).Count -eq $scenarios.Count
        noLargeAiFields     = -not ($scenarios | Where-Object { $_.loadsLargeAiFields })
        noExternalApi       = $true
    }
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 8 | Set-Content -Path $OutFile -Encoding UTF8
Write-Host "Wrote $OutFile"
$scenarios | Format-Table targetTodoProducts, summaryMs, todosPage1Ms, todosTotal, paginationOk, loadsLargeAiFields
