# F7 sensitiveConfirm static scan — core write pages should import sensitiveActions.
param(
    [string]$OutFile = "docs/demo-sensitive-confirm-scan.auto.json"
)

$repoRoot = Split-Path -Parent $PSScriptRoot
$checks = @(
    @{ file = "admin/src/pages/Product/DraftDetail/index.tsx"; pattern = "confirmPlatformPublishConfigSave" }
    @{ file = "admin/src/pages/Product/PublishBatch/index.tsx"; pattern = "sensitiveActions" }
    @{ file = "admin/src/pages/TaskCenter/Failures/index.tsx"; pattern = "sensitiveActions|confirmFailureTaskRetry" }
    @{ file = "admin/src/pages/Settings/Users/index.tsx"; pattern = "sensitiveActions" }
    @{ file = "admin/src/pages/Customer/ConversationDetail/index.tsx"; pattern = "confirmCustomerReplySend" }
    @{ file = "admin/src/pages/Shops/index.tsx"; pattern = "confirmRevokeStoreAuth" }
    @{ file = "admin/src/pages/Settings/Platforms/index.tsx"; pattern = "confirmPlatformConfigSave" }
    @{ file = "admin/src/pages/Settings/Storage/index.tsx"; pattern = "confirmStoragePublicTest" }
)

$results = @(); $failed = 0
foreach ($c in $checks) {
    $path = Join-Path $repoRoot $c.file
    $ok = $false
    if (Test-Path $path) {
        $content = Get-Content $path -Raw
        $ok = $content -match $c.pattern
    }
    if (-not $ok) { $failed++ }
    $results += @{ file = $c.file; status = $(if ($ok) { "passed" } else { "failed" }) }
}

$out = @{ phase = "F7"; generatedAt = (Get-Date).ToUniversalTime().ToString("o"); results = $results; failed = $failed }
$full = Join-Path $repoRoot $OutFile
$out | ConvertTo-Json -Depth 6 | Set-Content -Path $full -Encoding UTF8
Write-Host "sensitiveConfirm scan: failed=$failed"
exit $(if ($failed -gt 0) { 1 } else { 0 })
