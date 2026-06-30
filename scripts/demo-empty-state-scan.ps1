# F7 EmptyState static scan — list pages should import useListEmptyLocale or EmptyState.
param(
    [string]$OutFile = "docs/demo-empty-state-scan.auto.json"
)

$repoRoot = Split-Path -Parent $PSScriptRoot
$pagesRoot = Join-Path $repoRoot "admin/src/pages"
$required = @(
    "Dashboard/ProductOperations",
    "Collect/Hub",
    "Collect/Tasks",
    "Product/Drafts",
    "AI/OperationWorkbench",
    "AI/TextBatches",
    "AI/ImageBatches",
    "Product/PublishTasks",
    "Orders/index.tsx",
    "Orders/Exceptions",
    "Inventory/index.tsx",
    "Inventory/Alerts",
    "Inventory/Deductions",
    "Inventory/SyncTasks",
    "Customer/Hub",
    "Customer/Conversations",
    "TaskCenter/Failures",
    "Settings/ConfigStatus",
    "Settings/Users",
    "System/OperationLogs"
)

$results = @(); $failed = 0
foreach ($rel in $required) {
    $path = if ($rel.EndsWith(".tsx")) { Join-Path $pagesRoot $rel } else { Join-Path $pagesRoot "$rel/index.tsx" }
    $ok = $false
    if (Test-Path $path) {
        $content = Get-Content $path -Raw
        $ok = $content -match "useListEmptyLocale|EmptyState|buildListEmptyLocale"
    }
    if (-not $ok) { $failed++ }
    $results += @{ page = $rel; path = $path; status = $(if ($ok) { "passed" } else { "failed" }) }
}

$out = @{ phase = "F7"; generatedAt = (Get-Date).ToUniversalTime().ToString("o"); results = $results; failed = $failed }
$full = Join-Path $repoRoot $OutFile
$out | ConvertTo-Json -Depth 6 | Set-Content -Path $full -Encoding UTF8
Write-Host "EmptyState scan: failed=$failed"
exit $(if ($failed -gt 0) { 1 } else { 0 })
