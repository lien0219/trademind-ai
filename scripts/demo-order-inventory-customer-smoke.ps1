# F7 Order / Inventory / Customer deep-link smoke.
param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$OutFile = "docs/demo-order-inventory-customer-smoke.auto.json"
)

$ErrorActionPreference = "Continue"
$ApiV1 = "$ApiBase/api/v1"
$repoRoot = Split-Path -Parent $PSScriptRoot

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
        if (-not [string]::IsNullOrWhiteSpace($key) -and -not (Test-Path "env:$key")) { Set-Item -Path "env:$key" -Value $val }
    }
}
Import-DotEnv (Join-Path $repoRoot ".env")

$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $env:ADMIN_BOOTSTRAP_EMAIL; password = $env:ADMIN_BOOTSTRAP_PASSWORD } | ConvertTo-Json)
$token = $login.data.token
$h = @{ Authorization = "Bearer $token"; Accept = "application/json" }

$results = @(); $failed = 0
function Probe($name, $url) {
    try {
        $r = Invoke-RestMethod -Method Get -Uri $url -Headers $h -TimeoutSec 15
        $ok = $null -ne $r.data
        if (-not $ok) { $script:failed++ }
        $script:results += @{ name = $name; status = $(if ($ok) { "passed" } else { "failed" }); url = $url }
    } catch {
        $script:failed++
        $script:results += @{ name = $name; status = "failed"; url = $url; detail = $_.Exception.Message }
    }
}

Probe "orders_list" "$ApiV1/orders?page=1&pageSize=5"
Probe "order_exceptions" "$ApiV1/orders/exceptions?page=1&pageSize=5"
Probe "inventory_center" "$ApiV1/inventory?page=1&pageSize=5"
Probe "inventory_sync_tasks" "$ApiV1/inventory-sync/tasks?page=1&pageSize=5"
Probe "customer_conversations" "$ApiV1/customer/conversations?page=1&pageSize=5"
Probe "task_center_failures" "$ApiV1/task-center/failures?page=1&pageSize=5"

$out = @{ phase = "F7"; generatedAt = (Get-Date).ToUniversalTime().ToString("o"); results = $results; failed = $failed }
$path = Join-Path $repoRoot $OutFile
$out | ConvertTo-Json -Depth 6 | Set-Content -Path $path -Encoding UTF8
Write-Host "Wrote $path (failed=$failed)"
exit $(if ($failed -gt 0) { 1 } else { 0 })
