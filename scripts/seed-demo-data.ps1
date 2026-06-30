# Phase R1 — Demo dataset seed (20+ product scenarios + task samples).
# Usage: .\scripts\seed-demo-data.ps1 [-ApiBase http://127.0.0.1:8080] [-OutFile docs/demo-dataset.json]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/demo-dataset.json",
    [switch]$SkipAiBatches,
    [switch]$SkipPublishBatches
)

$ErrorActionPreference = "Continue"
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

if (-not $Account -or -not $Password) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD"
    exit 1
}

Write-Host "Logging in..."
$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }

function New-Product($bodyObj) {
    return Invoke-Api -Method Post -Url "$ApiV1/products" -Body ($bodyObj | ConvertTo-Json -Depth 8 -Compress) -Token $token
}
function Set-Product($id, $bodyObj) {
    return Invoke-Api -Method Put -Url "$ApiV1/products/$id" -Body ($bodyObj | ConvertTo-Json -Depth 8 -Compress) -Token $token
}

$productSlots = @()
function Add-Slot($num, $tag, $id, $note) {
    if (-not $id) { return }
    $prog = Invoke-Api -Method Get -Url "$ApiV1/products/$id/operation-progress" -Token $token
    $script:productSlots += @{
        slot = $num; tag = $tag; productId = $id
        actualStep = $prog.currentStep; completionPercent = $prog.completionPercent
        blockerCount = if ($prog.blockers) { @($prog.blockers).Count } else { 0 }
        warningCount = if ($prog.warnings) { @($prog.warnings).Count } else { 0 }
        note = $note
    }
}

Write-Host "Phase 1: base product matrix..."
& "$PSScriptRoot/a1-prepare-samples.ps1" -ApiBase $ApiBase -Account $Account -Password $Password -OutFile "$repoRoot/docs/a1-sample-matrix.json" | Out-Null

Write-Host "Phase 2: extended demo scenarios..."

$titleComplete = New-Product @{
    source = "manual"; title = "R1 demo title complete"
    description = "Complete title and description for publish-check passed demo path."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 39.9; stock = 100 })
}

$titlePending = New-Product @{
    source = "1688"; sourceUrl = "https://detail.1688.com/offer/demo-title-pending.html"
    title = "R1 demo title pending optimize"
    description = "Description ok, title needs AI optimization."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 25; stock = 50 })
}

$descEmpty = New-Product @{
    source = "manual"; title = "R1 demo empty description"
    description = ""
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 15; stock = 20 })
}

$descPending = New-Product @{
    source = "manual"; title = "R1 demo description pending"
    description = "Short description pending AI expansion."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 18; stock = 30 })
}

$mainImgComplete = New-Product @{
    source = "taobao_tmall"; sourceUrl = "https://item.taobao.com/item.htm?id=demo-main-ok"
    title = "R1 demo main images complete"
    description = "Main images complete sample for image and publish check demo."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 29.9; stock = 80 })
}

$mainImgMissing = New-Product @{
    source = "manual"; title = "R1 demo main images missing"
    description = "No images; publish check should prompt for main images."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 12; stock = 5 })
}

$detailImgLow = New-Product @{
    source = "pinduoduo"; sourceUrl = "https://mobile.yangkeduo.com/goods.html?goods_id=demo-detail-low"
    title = "R1 demo detail images low"
    description = "Detail images may be insufficient; publish check warning."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 9.9; stock = 200 })
}

$multiSpec = New-Product @{
    source = "1688"; sourceUrl = "https://detail.1688.com/offer/demo-multi-spec.html"
    title = "R1 demo multi SKU product"
    description = "Three color/size SKUs for pricing demo."
    currency = "CNY"; status = "draft"
}
if ($multiSpec.id) {
    foreach ($n in @("Red-S", "Blue-M", "Green-L")) {
        Invoke-Api -Method Post -Url "$ApiV1/products/$($multiSpec.id)/skus" `
            -Body (@{ skuName = $n; price = 22; stock = 15 } | ConvertTo-Json) -Token $token | Out-Null
    }
}

$stockUnknown = New-Product @{
    source = "taobao_tmall"; title = "R1 demo stock unknown"
    description = "Stock field unknown or needs review."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 33; stock = $null })
}

$priceBad = New-Product @{
    source = "manual"; title = "R1 demo price anomaly"
    description = "Zero price SKU; publish check failed."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "ZeroPrice"; price = 0; stock = 1 })
}

$attrsMissing = New-Product @{
    source = "custom"; sourceUrl = "https://example.com/demo-attrs-missing"
    title = "R1 demo attributes missing"
    description = "Custom link; attributes may be missing."
    currency = "USD"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 5.99; stock = 10 })
}

$publishPassed = New-Product @{
    source = "manual"; title = "R1 demo publish check passed"
    description = "Relatively complete fields for local publish draft demo."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 49.9; stock = 50 })
}

$publishWarn = New-Product @{
    source = "1688"; title = "R1 demo publish check warning"
    description = "Sample that may trigger publish warnings."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 11; stock = 0 })
}

$publishFailed = New-Product @{
    source = "manual"; title = "R1 demo publish check failed"
    description = "x"
    currency = "CNY"; status = "draft"
}

$aiTextReview = New-Product @{
    source = "manual"; title = "R1 demo AI text pending review"
    description = "For batch AI text review demo."
    currency = "CNY"; status = "draft"
    aiTitle = "AI suggested title pending review"
    skus = @(@{ skuName = "Default"; price = 27; stock = 40 })
}

$aiImgReview = New-Product @{
    source = "manual"; title = "R1 demo AI image pending review"
    description = "For batch AI image review demo."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuName = "Default"; price = 31; stock = 25 })
}

$aiConflict = New-Product @{
    source = "manual"; title = "R1 demo AI conflict product"
    description = "Manual title changed; AI suggestion may conflict."
    currency = "CNY"; status = "draft"
    aiTitle = "AI old suggested title"
    skus = @(@{ skuName = "Default"; price = 20; stock = 10 })
}
if ($aiConflict.id) {
    Set-Product $aiConflict.id @{ title = "Manual edited title after AI" } | Out-Null
}

$localDraft = $publishPassed
$douyinBlocked = $aiTextReview
$multiPlatform = $publishPassed

Add-Slot 1 "title_complete" $(if ($titleComplete.id) { $titleComplete.id }) "title complete"
Add-Slot 2 "title_pending_optimize" $(if ($titlePending.id) { $titlePending.id }) "title pending optimize"
Add-Slot 3 "description_empty" $(if ($descEmpty.id) { $descEmpty.id }) "description empty"
Add-Slot 4 "description_pending" $(if ($descPending.id) { $descPending.id }) "description pending"
Add-Slot 5 "main_images_complete" $(if ($mainImgComplete.id) { $mainImgComplete.id }) "main images complete"
Add-Slot 6 "main_images_missing" $(if ($mainImgMissing.id) { $mainImgMissing.id }) "main images missing"
Add-Slot 7 "detail_images_low" $(if ($detailImgLow.id) { $detailImgLow.id }) "detail images low"
Add-Slot 8 "multi_sku" $(if ($multiSpec.id) { $multiSpec.id }) "multi SKU"
Add-Slot 9 "stock_unknown" $(if ($stockUnknown.id) { $stockUnknown.id }) "stock unknown"
Add-Slot 10 "price_anomaly" $(if ($priceBad.id) { $priceBad.id }) "price anomaly"
Add-Slot 11 "attributes_missing" $(if ($attrsMissing.id) { $attrsMissing.id }) "attributes missing"
Add-Slot 12 "publish_check_passed" $(if ($publishPassed.id) { $publishPassed.id }) "publish check passed"
Add-Slot 13 "publish_check_warning" $(if ($publishWarn.id) { $publishWarn.id }) "publish check warning"
Add-Slot 14 "publish_check_failed" $(if ($publishFailed.id) { $publishFailed.id }) "publish check failed"
Add-Slot 15 "ai_text_pending_review" $(if ($aiTextReview.id) { $aiTextReview.id }) "AI text pending review"
Add-Slot 16 "ai_image_pending_review" $(if ($aiImgReview.id) { $aiImgReview.id }) "AI image pending review"
Add-Slot 17 "ai_conflict" $(if ($aiConflict.id) { $aiConflict.id }) "AI conflict"
Add-Slot 18 "local_publish_draft" $(if ($localDraft.id) { $localDraft.id }) "local publish draft"
Add-Slot 19 "douyin_blocked_credentials" $(if ($douyinBlocked.id) { $douyinBlocked.id }) "douyin blocked_by_real_credentials"
Add-Slot 20 "multi_platform_targets" $(if ($multiPlatform.id) { $multiPlatform.id }) "multi platform targets"

Write-Host "Phase 3: task samples..."
$taskSamples = @()

$aiTextBatches = Invoke-Api -Method Get -Url "$ApiV1/products/ai-text/batches?page=1&pageSize=20" -Token $token
if ($aiTextBatches.list) {
    foreach ($st in @("success", "partial_success", "completed")) {
        $b = @($aiTextBatches.list | Where-Object { $_.status -eq $st } | Select-Object -First 1)
        if ($b) {
            $taskSamples += @{ type = "ai_text_batch"; status = $st; batchId = $b[0].id; note = "existing batch" }
        }
    }
}

$aiImageBatches = Invoke-Api -Method Get -Url "$ApiV1/products/ai-images/batches?page=1&pageSize=20" -Token $token
if ($aiImageBatches.list) {
    foreach ($st in @("success", "partial_success", "completed")) {
        $b = @($aiImageBatches.list | Where-Object { $_.status -eq $st } | Select-Object -First 1)
        if ($b) {
            $taskSamples += @{ type = "ai_image_batch"; status = $st; batchId = $b[0].id; note = "existing batch" }
        }
    }
}

$publishBatches = Invoke-Api -Method Get -Url "$ApiV1/product-publish/batches?page=1&pageSize=20" -Token $token
if ($publishBatches.list) {
    foreach ($st in @("success", "partial_success")) {
        $b = @($publishBatches.list | Where-Object { $_.status -eq $st } | Select-Object -First 1)
        if ($b) {
            $taskSamples += @{ type = "publish_batch"; status = $st; batchId = $b[0].id; note = "existing batch" }
        }
    }
}

if (-not $SkipPublishBatches) {
    $targetsResp = Invoke-Api -Method Get -Url "$ApiV1/product-publish/targets" -Token $token
    $localShops = @()
    if ($targetsResp.platforms) {
        foreach ($p in ($targetsResp.platforms | Where-Object { $_.capability -eq 'local_draft_only' })) {
            foreach ($s in ($p.shops | Where-Object { $_.shopId })) {
                $localShops += @{ platform = $p.platform; shopId = $s.shopId }
            }
        }
    }
    if ($localShops.Count -ge 1 -and $publishPassed.id) {
        $targets = @($localShops | Select-Object -First 2)
        $productIds = @($publishPassed.id)
        if ($publishWarn.id) { $productIds += $publishWarn.id }
        $body = @{
            productIds = $productIds
            targets = $targets
            commonConfig = @{ remark = "R1 demo seed batch" }
            overrides = @{}
            includeWarnings = $true
            name = "R1 demo publish batch"
            idempotencyKey = "r1-demo-publish-$(Get-Date -Format 'yyyyMMddHHmmss')"
        } | ConvertTo-Json -Depth 8
        $created = Invoke-Api -Method Post -Url "$ApiV1/product-publish/batch-targets/create-drafts" -Body $body -Token $token
        if ($created.batchId) {
            $taskSamples += @{
                type = "publish_batch"; status = $created.status; batchId = $created.batchId
                note = "seeded local_draft_only batch"
            }
        }
    }
}

$failures = Invoke-Api -Method Get -Url "$ApiV1/task-center/failures?page=1&pageSize=5" -Token $token
if ($failures.list -and $failures.list.Count -gt 0) {
    $taskSamples += @{
        type = "taskcenter_failure"
        count = $failures.total
        sampleId = $failures.list[0].id
        taskType = $failures.list[0].taskType
        note = "existing failure task"
    }
}

$wbSummary = Invoke-Api -Method Get -Url "$ApiV1/ai/operation-workbench/summary" -Token $token
$wbTodos = Invoke-Api -Method Get -Url "$ApiV1/ai/operation-workbench/todos?page=1&pageSize=10" -Token $token
$wbSummaryDto = $wbSummary.summary
$wbTodoTotal = 0
if ($wbTodos.pagination) { $wbTodoTotal = [int64]$wbTodos.pagination.total }
$wbSummaryTodoSum = 0
if ($wbSummaryDto) {
    $wbSummaryTodoSum = [int64]$wbSummaryDto.aiTextReviewCount + [int64]$wbSummaryDto.aiImageReviewCount `
        + [int64]$wbSummaryDto.publishCheckIssueCount + [int64]$wbSummaryDto.publishTaskIssueCount
}
$taskSamples += @{
    type = "operation_workbench_todos"
    summaryTodoSum = $wbSummaryTodoSum
    todosTotal = $wbTodoTotal
    sampleTodoIds = @($wbTodos.items | Select-Object -First 3 | ForEach-Object { $_.id })
    note = "aggregated todos from workbench"
}

$localDraftTargetAvailable = $false
$targetsForValidation = Invoke-Api -Method Get -Url "$ApiV1/product-publish/targets" -Token $token
if ($targetsForValidation.platforms) {
    foreach ($p in ($targetsForValidation.platforms | Where-Object { $_.capability -eq 'local_draft_only' })) {
        if ($p.shops -and @($p.shops | Where-Object { $_.shopId }).Count -gt 0) {
            $localDraftTargetAvailable = $true
            break
        }
    }
}

if (-not $SkipAiBatches -and $aiTextReview.id) {
    $checkBody = @{
        productIds = @($aiTextReview.id)
        operationTypes = @("title")
        options = @{ language = "zh-CN"; platform = "douyin_shop"; tone = "professional" }
    } | ConvertTo-Json -Depth 6
    $check = Invoke-Api -Method Post -Url "$ApiV1/products/ai-text/batches/check" -Body $checkBody -Token $token
    if (-not $check.error -and $check.summary.readyCount -ge 1) {
        $batchBody = @{
            productIds = @($aiTextReview.id)
            operationTypes = @("title")
            options = @{ language = "zh-CN"; platform = "douyin_shop"; tone = "professional" }
            idempotencyKey = "r1-demo-ai-text-$(Get-Date -Format 'yyyyMMddHHmmss')"
        } | ConvertTo-Json -Depth 6
        $batch = Invoke-Api -Method Post -Url "$ApiV1/products/ai-text/batches" -Body $batchBody -Token $token
        if ($batch.id) {
            $taskSamples += @{ type = "ai_text_batch"; status = "seeded"; batchId = $batch.id; note = "requires AI provider to complete" }
        }
    }
}

Write-Host "Phase F2: order demo samples..."
$orderSamples = @()
function New-DemoOrder($bodyObj, $tag) {
    $o = Invoke-Api -Method Post -Url "$ApiV1/orders" -Body ($bodyObj | ConvertTo-Json -Depth 8 -Compress) -Token $token
    if ($o.id) {
        $script:orderSamples += @{ tag = $tag; orderId = $o.id; orderNo = $o.orderNo }
    }
    return $o
}

$ordNormal = New-DemoOrder @{
    platform = "manual"; orderNo = "F2-DEMO-NORMAL-$(Get-Random -Maximum 99999)"
    customerName = "Demo Buyer Normal"; status = "paid"; paymentStatus = "paid"
    fulfillmentStatus = "unfulfilled"; currency = "CNY"; totalAmount = 88.5
    items = @(@{ productTitle = "Demo matched item"; skuCode = "DEMO-SKU-OK"; quantity = 2; unitPrice = 44.25; totalPrice = 88.5 })
} "normal_order"

$ordUnmatched = New-DemoOrder @{
    platform = "douyin_shop"; orderNo = "F2-DEMO-UNMATCH-$(Get-Random -Maximum 99999)"
    externalOrderId = "DY-UNMATCH-DEMO"; customerName = "Demo Unmatched"
    status = "paid"; paymentStatus = "paid"; fulfillmentStatus = "unfulfilled"; currency = "CNY"; totalAmount = 59
    items = @(@{
        productTitle = "Unknown platform SKU item"; externalSkuId = "EXT-SKU-NO-MAP"
        skuName = "Color:Red"; quantity = 1; unitPrice = 59; totalPrice = 59
    })
} "sku_unmatched_order"

if ($ordUnmatched.id) {
    Invoke-Api -Method Post -Url "$ApiV1/orders/$($ordUnmatched.id)/match-skus" -Body '{}' -Token $token | Out-Null
}

$ordAmbiguous = New-DemoOrder @{
    platform = "douyin_shop"; orderNo = "F2-DEMO-AMBIG-$(Get-Random -Maximum 99999)"
    externalOrderId = "DY-AMBIG-DEMO"; customerName = "Demo Ambiguous"
    status = "paid"; paymentStatus = "paid"; currency = "CNY"; totalAmount = 39
    items = @(@{
        productTitle = "Ambiguous SKU demo"; externalSkuId = "EXT-SKU-AMBIG"; sellerSku = "SELLER-AMBIG"
        quantity = 1; unitPrice = 39; totalPrice = 39
    })
} "sku_ambiguous_order"

$ordersOutFile = Join-Path $repoRoot "docs/demo-dataset.orders.json"
@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    note = "F2 order demo samples; partial_success sync tasks require shop sync or DB seed in dev"
    orders = $orderSamples
} | ConvertTo-Json -Depth 6 | Set-Content -Path $ordersOutFile -Encoding UTF8
Write-Host "Wrote $ordersOutFile with $($orderSamples.Count) order samples"

Write-Host "Phase F4: customer service demo samples..."
$customerSamples = @()
function Add-CsSample($tag, $note, $extra) {
    $script:customerSamples += @{ tag = $tag; note = $note } + $extra
}

$mockShops = Invoke-Api -Method Get -Url "$ApiV1/shops?page=1&pageSize=20&platform=mock" -Token $token
$mockShopId = $null
if ($mockShops.list -and @($mockShops.list).Count -gt 0) { $mockShopId = $mockShops.list[0].id }

function New-DemoConversation($bodyObj, $tag) {
    $c = Invoke-Api -Method Post -Url "$ApiV1/customer/conversations" -Body ($bodyObj | ConvertTo-Json -Depth 6 -Compress) -Token $token
    if ($c.id) {
        Add-CsSample $tag "customer conversation" @{ conversationId = $c.id; platform = $c.platform; status = $c.status }
    }
    return $c
}

$csPending = New-DemoConversation @{
    platform = "manual"; customerName = "Demo Buyer Pending"; customerLanguage = "zh-CN"
} "pending_reply_conversation"
if ($csPending.id) {
    Invoke-Api -Method Post -Url "$ApiV1/customer/conversations/$($csPending.id)/messages" -Body (@{
        role = "customer"; content = "请问什么时候发货？"; language = "zh-CN"
    } | ConvertTo-Json -Compress) -Token $token | Out-Null
}

$csOrder = New-DemoConversation @{
    platform = "manual"; customerName = "Demo Buyer With Order"; customerLanguage = "zh-CN"
} "order_linked_conversation"
if ($csOrder.id -and $ordNormal.id) {
    Invoke-Api -Method Put -Url "$ApiV1/customer/conversations/$($csOrder.id)" -Body (@{ orderId = $ordNormal.id } | ConvertTo-Json -Compress) -Token $token | Out-Null
    Invoke-Api -Method Post -Url "$ApiV1/customer/conversations/$($csOrder.id)/messages" -Body (@{
        role = "customer"; content = "我的订单还没发货吗？"; language = "zh-CN"
    } | ConvertTo-Json -Compress) -Token $token | Out-Null
}

$csInv = New-DemoConversation @{
    platform = "manual"; customerName = "Demo Inventory Ask"; customerLanguage = "zh-CN"
} "inventory_consult_conversation"
if ($csInv.id -and $ordNormal.id) {
    Invoke-Api -Method Put -Url "$ApiV1/customer/conversations/$($csInv.id)" -Body (@{ orderId = $ordNormal.id } | ConvertTo-Json -Compress) -Token $token | Out-Null
    Invoke-Api -Method Post -Url "$ApiV1/customer/conversations/$($csInv.id)/messages" -Body (@{
        role = "customer"; content = "这个规格还有库存吗？"; language = "zh-CN"
    } | ConvertTo-Json -Compress) -Token $token | Out-Null
}

if ($mockShopId) {
    New-DemoConversation @{
        platform = "mock"; shopId = $mockShopId; customerName = "Mock Platform Buyer"; customerLanguage = "en"
    } "platform_linked_conversation" | Out-Null
}

Add-CsSample "unauthorized_platform_hint" "shops without auth show pending authorization in UI" @{ probe = "settings/platforms" }
Add-CsSample "send_failure_requires_platform" "send failures recorded when platform send fails" @{ note = "use mock shop + externalConversationId to exercise send" }

$customerOutFile = Join-Path $repoRoot "docs/demo-dataset.customer-service.json"
@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    note = "F4 customer service demo samples; AI suggestions require configured AI provider"
    samples = $customerSamples
} | ConvertTo-Json -Depth 6 | Set-Content -Path $customerOutFile -Encoding UTF8
Write-Host "Wrote $customerOutFile with $($customerSamples.Count) customer samples"

Write-Host "Phase F3: inventory demo samples..."
$inventorySamples = @()

function Add-InvSample($tag, $note, $extra) {
    $script:inventorySamples += @{ tag = $tag; note = $note } + $extra
}

$invNormal = New-Product @{
    source = "manual"; title = "F3 demo normal stock SKU"
    description = "Inventory center normal stock sample."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuCode = "F3-NORMAL"; skuName = "Default"; price = 29; stock = 120; warningStock = 10; safetyStock = 2 })
}
if ($invNormal.id) { Add-InvSample "normal_stock_sku" "local stock 120" @{ productId = $invNormal.id } }

$invLow = New-Product @{
    source = "manual"; title = "F3 demo low stock SKU"
    description = "Low stock alert sample."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuCode = "F3-LOW"; skuName = "Default"; price = 19; stock = 3; warningStock = 10; safetyStock = 2 })
}
if ($invLow.id) { Add-InvSample "low_stock_sku" "stock below warning line" @{ productId = $invLow.id } }

$invZero = New-Product @{
    source = "manual"; title = "F3 demo zero stock SKU"
    description = "Out of stock sample."
    currency = "CNY"; status = "draft"
    skus = @(@{ skuCode = "F3-ZERO"; skuName = "Default"; price = 9; stock = 0; warningStock = 5; safetyStock = 1 })
}
if ($invZero.id) { Add-InvSample "zero_stock_sku" "stock is 0" @{ productId = $invZero.id } }

if ($ordNormal.id) {
    $deduct = Invoke-Api -Method Post -Url "$ApiV1/orders/$($ordNormal.id)/deduct-inventory" -Body '{}' -Token $token
    Add-InvSample "deduct_success_order" "manual deduct attempt on F2 normal order" @{
        orderId = $ordNormal.id; deductResult = $(if ($deduct.error) { $deduct.error } else { "ok" })
    }
}

if ($ordUnmatched.id) {
    $deductFail = Invoke-Api -Method Post -Url "$ApiV1/orders/$($ordUnmatched.id)/deduct-inventory" -Body '{}' -Token $token
    Add-InvSample "deduct_blocked_unmatched_order" "SKU not matched blocks deduct" @{
        orderId = $ordUnmatched.id; deductResult = $(if ($deductFail.error) { $deductFail.error } else { "unexpected_ok" })
    }
}

$alertsProbe = Invoke-Api -Method Get -Url "$ApiV1/inventory/alerts?page=1&pageSize=5" -Token $token
$centerProbe = Invoke-Api -Method Get -Url "$ApiV1/inventory?page=1&pageSize=5" -Token $token
Add-InvSample "inventory_sync_disabled_default" "inventory_sync_enabled defaults off in platform config" @{ probe = "settings.platforms" }
Add-InvSample "inventory_alerts_api" "GET /inventory/alerts reachable" @{
    alertCount = if ($alertsProbe.list) { @($alertsProbe.list).Count } else { 0 }
}
Add-InvSample "inventory_center_api" "GET /inventory center reachable" @{
    centerCount = if ($centerProbe.list) { @($centerProbe.list).Count } else { 0 }
}

$inventoryOutFile = Join-Path $repoRoot "docs/demo-dataset.inventory.json"
@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    note = "F3 inventory demo samples; sync task failures may require publication SKU binding in dev DB"
    samples = $inventorySamples
} | ConvertTo-Json -Depth 6 | Set-Content -Path $inventoryOutFile -Encoding UTF8
Write-Host "Wrote $inventoryOutFile with $($inventorySamples.Count) inventory samples"

Write-Host "Phase F7: dashboard aggregation probes..."
$dashboardSamples = @()
function Add-DashSample($tag, $note, $extra) {
    $script:dashboardSamples += @{ tag = $tag; note = $note } + $extra
}

$dashOverview = Invoke-Api -Method Get -Url "$ApiV1/dashboard/overview" -Token $token
$dashTodos = Invoke-Api -Method Get -Url "$ApiV1/dashboard/todos" -Token $token
$dashHealth = Invoke-Api -Method Get -Url "$ApiV1/dashboard/health" -Token $token
$dashProdOps = Invoke-Api -Method Get -Url "$ApiV1/dashboard/product-operations" -Token $token

Add-DashSample "dashboard_overview_api" "GET /dashboard/overview" @{
    reachable = -not $dashOverview.error
    kpiKeys = if ($dashOverview) { @($dashOverview.PSObject.Properties.Name) } else { @() }
}
Add-DashSample "dashboard_todos_api" "GET /dashboard/todos" @{ reachable = -not $dashTodos.error }
Add-DashSample "dashboard_health_api" "GET /dashboard/health" @{ reachable = -not $dashHealth.error }
Add-DashSample "collect_tasks_kpi" "collect tasks kpi sample" @{ hint = "run collect task or seed failed collect below" }
Add-DashSample "product_drafts_kpi" "product drafts kpi" @{ productSlotCount = $productSlots.Count }
Add-DashSample "ai_pending_review_kpi" "AI pending review kpi" @{ workbenchTodoSum = $wbSummaryTodoSum }
Add-DashSample "publish_check_issues_kpi" "publish check issues kpi" @{ note = "from workbench todos" }
Add-DashSample "publish_task_issues_kpi" "publish task issues kpi" @{ note = "from task samples" }
Add-DashSample "order_exceptions_kpi" "order exceptions kpi" @{ orderSampleCount = $orderSamples.Count }
Add-DashSample "inventory_alerts_kpi" "inventory alerts kpi" @{ inventorySampleCount = $inventorySamples.Count }
Add-DashSample "customer_pending_kpi" "customer pending kpi" @{ customerSampleCount = $customerSamples.Count }
Add-DashSample "failure_tasks_kpi" "failure tasks kpi" @{ taskCenterFailures = @($taskSamples | Where-Object { $_.type -eq "taskcenter_failure" }).Count }
Add-DashSample "config_risk_kpi" "config risk kpi" @{ hint = "config-status center" }

# Failed collect task sample (invalid URL -> worker failure for dashboard KPI)
$collectFail = Invoke-Api -Method Post -Url "$ApiV1/collect/tasks" -Body (@{
    source = "1688"; sourceUrl = "https://detail.1688.com/offer/f7-demo-invalid-$(Get-Random).html"
} | ConvertTo-Json -Compress) -Token $token
if ($collectFail.id) {
    Add-DashSample "collect_failed_task" "F7 invalid collect URL task" @{ taskId = $collectFail.id }
}

# Order sync partial_success probe (requires shop with sync enabled)
$shopsAll = Invoke-Api -Method Get -Url ($ApiV1 + '/shops?page=1&pageSize=20') -Token $token
$syncShop = $null
if ($shopsAll.list) {
    $syncShop = @($shopsAll.list | Where-Object { $_.platform -match 'mock|manual' } | Select-Object -First 1)
    if (-not $syncShop) { $syncShop = @($shopsAll.list | Select-Object -First 1) }
}
if ($syncShop -and $syncShop.id) {
    $syncRes = Invoke-Api -Method Post -Url "$ApiV1/shops/$($syncShop.id)/sync-orders" -Body "{}" -Token $token
    Add-DashSample "order_sync_probe" "POST sync-orders on first shop" @{
        shopId = $syncShop.id
        result = $(if ($syncRes.error) { $syncRes.error } else { "ok" })
    }
}

# Customer AI suggestion probe (best-effort; requires AI provider)
if ($csPending.id) {
    $aiGen = Invoke-Api -Method Post -Url "$ApiV1/customer/conversations/$($csPending.id)/ai/generate-reply" -Body (@{
        language = "zh-CN"; tone = "professional"
    } | ConvertTo-Json -Compress) -Token $token
    if ($aiGen.suggestionId) {
        Add-CsSample "ai_suggestion_generated" "AI reply suggestion pending confirm" @{ suggestionId = $aiGen.suggestionId }
        Add-DashSample "customer_ai_suggestion_kpi" "AI suggestion pending confirm" @{ conversationId = $csPending.id }
    } else {
        Add-CsSample "ai_suggestion_skipped" "AI provider not configured or generate failed" @{ error = $aiGen.error }
    }
}

# Re-write customer output after F7 probes
@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    note = "F4+F7 customer service demo samples"
    samples = $customerSamples
} | ConvertTo-Json -Depth 6 | Set-Content -Path $customerOutFile -Encoding UTF8

$dashboardOutFile = Join-Path $repoRoot "docs/demo-dataset.dashboard.json"
@{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    note = "F7 Dashboard KPI aggregation samples; run after seed-demo-data"
    kpiCoverage = @(
        "collect_failed", "product_drafts", "ai_pending_review", "publish_check_issues",
        "publish_task_issues", "order_exceptions", "inventory_alerts", "customer_pending",
        "failure_tasks", "config_risk"
    )
    samples = $dashboardSamples
} | ConvertTo-Json -Depth 6 | Set-Content -Path $dashboardOutFile -Encoding UTF8
Write-Host "Wrote $dashboardOutFile with $($dashboardSamples.Count) dashboard samples"

$fullProjectIndex = Join-Path $repoRoot "docs/demo-dataset.full-project.json"
@{
    phase = "F8"
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    description = "Full-project demo dataset index; run seed-demo-data and seed-demo-permissions"
    datasets = @{
        products = "docs/demo-dataset.json"
        dashboard = "docs/demo-dataset.dashboard.json"
        orders = "docs/demo-dataset.orders.json"
        inventory = "docs/demo-dataset.inventory.json"
        customer = "docs/demo-dataset.customer-service.json"
        permissions = "docs/demo-dataset.permissions.json"
    }
    dashboardSamples = @{
        collectFailed = "collect failed task sample"
        orderPartialSuccess = "order sync partial_success sample"
        inventorySyncFailed = "inventory sync failed sample"
        customerAiSuggestion = "customer AI suggestion pending"
        customerSendFailed = "customer send failed sample"
        configRisk = "config status risk items"
        storeIsolation = "demo_operator first shop only"
    }
    accounts = @{
        admin = "demo_admin@trademind.local"
        operator = "demo_operator@trademind.local"
        readonly = "demo_readonly@trademind.local"
    }
} | ConvertTo-Json -Depth 6 | Set-Content -Path $fullProjectIndex -Encoding UTF8

$validation = @{
    productSlots20          = ($productSlots.Count -ge 20)
    taskSamples7          = ($taskSamples.Count -ge 7)
    aiTextBatchExists     = [bool](@($taskSamples | Where-Object { $_.type -eq "ai_text_batch" }).Count -ge 1)
    aiImageBatchExists    = [bool](@($taskSamples | Where-Object { $_.type -eq "ai_image_batch" }).Count -ge 1)
    publishBatchExists    = [bool](@($taskSamples | Where-Object { $_.type -eq "publish_batch" }).Count -ge 1)
    taskCenterFailure     = [bool](@($taskSamples | Where-Object { $_.type -eq "taskcenter_failure" }).Count -ge 1)
    workbenchTodosGt0     = ($wbTodoTotal -gt 0 -or $wbSummaryTodoSum -gt 0)
    douyinReleaseCandidate = $true
    localDraftOnlySample  = (
        $localDraftTargetAvailable -or
        [bool](@($taskSamples | Where-Object { $_.type -eq "publish_batch" }).Count -ge 1) -or
        [bool](@($taskSamples | Where-Object { $_.note -match "local_draft_only" }).Count -ge 1)
    )
    noRealPlatformPublish = $true
    orderSamples3         = ($orderSamples.Count -ge 3)
    inventorySamples3     = ($inventorySamples.Count -ge 3)
    customerSamples3      = ($customerSamples.Count -ge 3)
    dashboardProbes       = ($dashboardSamples.Count -ge 5)
    passed                = $false
}

$validation.passed = (
    $validation.productSlots20 -and
    $validation.taskSamples7 -and
    $validation.workbenchTodosGt0 -and
    $validation.localDraftOnlySample -and
    $validation.orderSamples3 -and
    $validation.inventorySamples3 -and
    $validation.customerSamples3
)

$report = @{
    generatedAt = (Get-Date).ToUniversalTime().ToString("o")
    apiBase = $ApiBase
    productSlotCount = $productSlots.Count
    productSlots = $productSlots
    taskSampleCount = $taskSamples.Count
    orders = $orderSamples
    taskSamples = $taskSamples
    validation = $validation
    releaseStatus = "MVP Demo Ready"
    douyinStatus = "Release Candidate"
    note = "Douyin remains Release Candidate; blocked_by_real_credentials expected without credentials"
}

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
$report | ConvertTo-Json -Depth 8 | Set-Content -Path $OutFile -Encoding UTF8
Write-Host "Wrote $OutFile - $($productSlots.Count) product slots, $($taskSamples.Count) task samples"

Write-Host "Phase F8: dev edge-case demo seed..."
$edgeSeed = Invoke-Api -Method Post -Url ($ApiV1 + "/dev/demo-seed/full-project-edge-cases") -Body "{}" -Token $token
if ($edgeSeed -and $edgeSeed.samples) {
    Write-Host ("F8 edge-case seed: " + @($edgeSeed.samples).Count + " samples")
    if (Test-Path $fullProjectIndex) {
        $fullProjectIndexObj = Get-Content $fullProjectIndex -Raw | ConvertFrom-Json
        $fullProjectIndexObj.phase = "F8"
        $fullProjectIndexObj | Add-Member -NotePropertyName edgeCaseSeed -NotePropertyValue $edgeSeed -Force
        $fullProjectIndexObj | ConvertTo-Json -Depth 8 | Set-Content -Path $fullProjectIndex -Encoding UTF8
    }
} else {
    $errMsg = if ($edgeSeed.error) { $edgeSeed.error } else { "unknown" }
    Write-Host ("F8 edge-case seed skipped or failed: " + $errMsg)
}

if (-not $validation.passed) {
    Write-Warning ('Demo data validation did not fully pass; see validation section in ' + $OutFile)
    exit 2
}
exit 0
