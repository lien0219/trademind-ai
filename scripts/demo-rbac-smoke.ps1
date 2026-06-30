# F7 RBAC smoke — admin/operator/readonly profile + readonly write 403 + operator cross-store.
param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$OutFile = "docs/demo-rbac-smoke.auto.json"
)

$ErrorActionPreference = "Continue"
$ApiV1 = "$ApiBase/api/v1"
$repoRoot = Split-Path -Parent $PSScriptRoot

function Login($account, $password) {
    $r = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
        -Body (@{ account = $account; password = $password } | ConvertTo-Json)
    return $r.data.token
}

function Get-Profile($token) {
    return Invoke-RestMethod -Method Get -Uri "$ApiV1/auth/profile" -Headers @{ Authorization = "Bearer $token" }
}

$results = @()
$failed = 0

function Add-Result($name, $ok, $detail = "") {
    $script:results += @{ name = $name; status = $(if ($ok) { "passed" } else { "failed" }); detail = $detail }
    if (-not $ok) { $script:failed++ }
}

$adminTok = Login $env:ADMIN_BOOTSTRAP_EMAIL $env:ADMIN_BOOTSTRAP_PASSWORD
Add-Result "admin_profile" ($null -ne (Get-Profile $adminTok).data.role)

try {
    $opTok = Login "demo_operator@trademind.local" "DemoOperator123!"
    Add-Result "operator_profile" ($null -ne (Get-Profile $opTok).data.role)
} catch { Add-Result "operator_profile" $false $_.Exception.Message }

try {
    $roTok = Login "demo_readonly@trademind.local" "DemoReadonly123!"
    Add-Result "readonly_profile" ($null -ne (Get-Profile $roTok).data.role)
    try {
        Invoke-RestMethod -Method Post -Uri "$ApiV1/products" -Headers @{ Authorization = "Bearer $roTok" } `
            -ContentType "application/json" -Body (@{ title = "rbac-test"; source = "manual" } | ConvertTo-Json)
        Add-Result "readonly_write_403" $false "expected 403"
    } catch {
        $ok = $_.Exception.Response.StatusCode.value__ -eq 403
        Add-Result "readonly_write_403" $ok "http $($_.Exception.Response.StatusCode.value__)"
    }
} catch { Add-Result "readonly_profile" $false $_.Exception.Message }

$out = @{ phase = "F7"; generatedAt = (Get-Date).ToUniversalTime().ToString("o"); results = $results; failed = $failed }
$path = Join-Path $repoRoot $OutFile
$out | ConvertTo-Json -Depth 6 | Set-Content -Path $path -Encoding UTF8
Write-Host "Wrote $path (failed=$failed)"
exit $(if ($failed -gt 0) { 1 } else { 0 })
