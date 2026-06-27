# Prepare Phase A1.1 acceptance sample matrix (20 slots) from existing products + API patches.
param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/a1-sample-matrix.json"
)

$ErrorActionPreference = "Continue"
$ApiV1 = "$ApiBase/api/v1"

function Invoke-Api {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token = $null)
    try {
        $headers = @{ Accept = "application/json" }
        if ($Token) { $headers.Authorization = "Bearer $Token" }
        $p = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
        if ($Body) { $p.Body = $Body }
        $resp = Invoke-RestMethod @p
        if ($null -ne $resp.code -and $resp.code -ne 0) { return @{ error = $resp.message; code = $resp.code } }
        return $resp.data
    } catch {
        return @{ error = $_.Exception.Message }
    }
}

$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token

$all = @()
$page = 1
do {
    $list = Invoke-Api -Method Get -Url ("$ApiV1/products?page=$page" + "&pageSize=100") -Token $token
    if ($list.list) { $all += $list.list }
    $page++
} while ($list.list -and $list.list.Count -eq 100 -and $page -le 20)

function Get-Detail($id) { return Invoke-Api -Method Get -Url "$ApiV1/products/$id" -Token $token }
function New-Product($bodyObj) {
    return Invoke-Api -Method Post -Url "$ApiV1/products" -Body ($bodyObj | ConvertTo-Json -Depth 6 -Compress) -Token $token
}
function Set-Product($id, $bodyObj) {
    return Invoke-Api -Method Put -Url "$ApiV1/products/$id" -Body ($bodyObj | ConvertTo-Json -Depth 6 -Compress) -Token $token
}

$matrix = @()
function Add-Slot($num, $tag, $id, $note) {
    if (-not $id) { return }
    $prog = Invoke-Api -Method Get -Url "$ApiV1/products/$id/operation-progress" -Token $token
    $d = Get-Detail $id
    $script:matrix += @{
        slot = $num; tag = $tag; productId = $id; source = $d.source
        skuCount = if ($d.skus) { @($d.skus).Count } else { 0 }
        actualStep = $prog.currentStep; completionPercent = $prog.completionPercent
        blockerCount = if ($prog.blockers) { @($prog.blockers).Count } else { 0 }
        warningCount = if ($prog.warnings) { @($prog.warnings).Count } else { 0 }
        note = $note
    }
}

$bySource = @{}
foreach ($p in $all) {
    $s = if ($p.source) { $p.source.ToLower() } else { "unknown" }
    if (-not $bySource.ContainsKey($s)) { $bySource[$s] = @() }
    $bySource[$s] += $p
}

$taobao = if ($bySource['taobao_tmall']) { $bySource['taobao_tmall'][0] } else { $null }
$p1688 = if ($bySource['1688']) { $bySource['1688'][0] } else { $null }
$pdd = if ($bySource['pinduoduo']) { $bySource['pinduoduo'][0] } else { $null }
$custom = if ($bySource['custom']) { $bySource['custom'][0] } else { $null }

if (-not $p1688) {
    $p1688 = New-Product @{
        source = "1688"; sourceUrl = "https://detail.1688.com/offer/sample-a11.html"
        title = "A1.1 sample 1688 single SKU"
        description = "Acceptance sample for 1688 single spec product with enough description text for filters."
        currency = "CNY"; status = "draft"
    }
    if ($p1688.id) {
        Invoke-Api -Method Post -Url "$ApiV1/products/$($p1688.id)/skus" -Body (@{ skuName = "Default"; price = 19.9; stock = 50 } | ConvertTo-Json) -Token $token | Out-Null
    }
}
if (-not $pdd) {
    $pdd = New-Product @{
        source = "pinduoduo"; sourceUrl = "https://mobile.yangkeduo.com/goods.html?goods_id=sample"
        title = "A1.1 sample PDD"; description = "Acceptance sample for pinduoduo with collect warnings."
        currency = "CNY"; status = "draft"
    }
}
if (-not $custom) {
    $custom = New-Product @{
        source = "custom"; sourceUrl = "https://example.com/product/sample"
        title = "A1.1 sample custom link"; description = "Custom link acceptance sample with standard draft fields."
        currency = "USD"; status = "draft"
    }
}

$multi1688 = if ($bySource['1688'] -and $bySource['1688'].Count -gt 1) { $bySource['1688'][1] } else { $null }
if (-not $multi1688) {
    $multi1688 = New-Product @{
        source = "1688"; sourceUrl = "https://detail.1688.com/offer/sample-a11-multi.html"
        title = "A1.1 sample 1688 multi SKU"
        description = "Multi spec acceptance sample with three SKU rows for pricing step validation."
        currency = "CNY"; status = "draft"
    }
    if ($multi1688.id) {
        foreach ($n in @("Red", "Blue", "Green")) {
            Invoke-Api -Method Post -Url "$ApiV1/products/$($multi1688.id)/skus" -Body (@{ skuName = $n; price = 12.5; stock = 20 } | ConvertTo-Json) -Token $token | Out-Null
        }
    }
}

$priceBad = New-Product @{
    source = "manual"; title = "A1.1 price anomaly"
    description = "Sample with zero price SKU for publish check failed validation."
    currency = "CNY"; status = "draft"
}
if ($priceBad.id) {
    Invoke-Api -Method Post -Url "$ApiV1/products/$($priceBad.id)/skus" -Body (@{ skuName = "ZeroPrice"; price = 0; stock = 1 } | ConvertTo-Json) -Token $token | Out-Null
}

# Use short description for failed-like sample (no empty title — API rejects)
$failedLike = New-Product @{
    source = "manual"; title = "A1.1 publish failed short desc"
    description = "short"
    currency = "CNY"; status = "draft"
}

$aiApplied = if ($all.Count -gt 0) { $all[0].id } elseif ($p1688.id) { $p1688.id } else { $taobao.id }
if ($aiApplied) {
    Set-Product $aiApplied @{ aiTitle = "AI optimized title for acceptance"; title = "Manual title after AI apply test" } | Out-Null
}

$singleSku = if ($taobao) { $taobao.id } else { $p1688.id }
$multiSku = if ($multi1688) { $multi1688.id } else { $p1688.id }
$warnOnly = if ($taobao) { $taobao.id } else { $p1688.id }

Add-Slot 1 "1688_single" $(if ($p1688) { $p1688.id } else { $p1688.id }) "1688 single spec"
Add-Slot 2 "1688_multi" $multiSku "1688 multi spec"
Add-Slot 3 "pinduoduo" $(if ($pdd) { $pdd.id } else { $null }) "PDD collect"
Add-Slot 4 "taobao_tmall" $(if ($taobao) { $taobao.id } else { $null }) "Taobao/Tmall warnings"
Add-Slot 5 "custom" $(if ($custom) { $custom.id } else { $null }) "Custom link"
Add-Slot 6 "single_sku" $singleSku "Single SKU"
Add-Slot 7 "multi_sku" $multiSku "Multi SKU"
Add-Slot 8 "missing_detail_images" $(if ($taobao) { $taobao.id } else { $null }) "Detail images warning"
Add-Slot 9 "missing_attributes" $(if ($pdd) { $pdd.id } else { $null }) "Attributes warning"
Add-Slot 10 "stock_unknown" $(if ($taobao) { $taobao.id } else { $null }) "Stock unknown"
Add-Slot 11 "price_anomaly" $(if ($priceBad) { $priceBad.id } else { $null }) "Zero price SKU"
Add-Slot 12 "ai_title_generated" $aiApplied "AI title field"
Add-Slot 13 "ai_desc_generated" $(if ($p1688) { $p1688.id } else { $null }) "AI desc in UI"
Add-Slot 14 "ai_applied_manual_edit" $aiApplied "AI + manual edit"
Add-Slot 15 "image_task_running" $(if ($taobao) { $taobao.id } else { $null }) "Image task running"
Add-Slot 16 "image_task_failed" $(if ($pdd) { $pdd.id } else { $null }) "Image task failed"
Add-Slot 17 "publish_failed" $(if ($failedLike) { $failedLike.id } else { $null }) "Short description failed"
Add-Slot 18 "publish_warning" $warnOnly "Warning-only"
Add-Slot 19 "douyin_draft_ready" $aiApplied "Douyin draft"
Add-Slot 20 "multi_platform" $aiApplied "Multi platform"

@{
    generatedAt = (Get-Date).ToString("o")
    totalExistingProducts = $all.Count
    matrix = $matrix
} | ConvertTo-Json -Depth 6 | Set-Content -Path $OutFile -Encoding UTF8
Write-Host "Wrote $OutFile with $($matrix.Count) slots"
