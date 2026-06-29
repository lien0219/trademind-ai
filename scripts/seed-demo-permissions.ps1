# Phase F5 — Demo RBAC accounts (run after main seed + at least one shop exists).
# Usage: .\scripts\seed-demo-permissions.ps1 [-ApiBase http://127.0.0.1:8080]

param(
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$AdminEmail = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$AdminPassword = $env:ADMIN_BOOTSTRAP_PASSWORD,
    [string]$OutFile = "docs/demo-dataset.permissions.json"
)

$ApiV1 = "$ApiBase/api/v1"

function Invoke-Api {
    param([string]$Method, [string]$Url, [string]$Body = $null, [string]$Token = $null)
    $headers = @{ Accept = "application/json" }
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $p = @{ Method = $Method; Uri = $Url; Headers = $headers; ContentType = "application/json" }
    if ($Body) { $p.Body = $Body }
    return Invoke-RestMethod @p
}

if (-not $AdminEmail -or -not $AdminPassword) {
    Write-Error "Set ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_PASSWORD"
    exit 1
}

$login = Invoke-Api -Method Post -Url "$ApiV1/auth/login" -Body (@{
    account = $AdminEmail; password = $AdminPassword
} | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "admin login failed"; exit 1 }

$shops = Invoke-Api -Method Get -Url "$ApiV1/shops?page=1&pageSize=10" -Token $token
$authorizedShop = $shops.data.list | Select-Object -First 1
$deniedShop = $shops.data.list | Select-Object -Skip 1 -First 1

function Ensure-User($email, $password, $displayName, $role) {
    try {
        $u = Invoke-Api -Method Post -Url "$ApiV1/admin/users" -Token $token -Body (@{
            email = $email; password = $password; displayName = $displayName; role = $role
        } | ConvertTo-Json)
        return $u.data
    } catch {
        $list = Invoke-Api -Method Get -Url "$ApiV1/admin/users?keyword=$email" -Token $token
        return $list.data.list | Where-Object { $_.email -eq $email } | Select-Object -First 1
    }
}

$demoAdmin = Ensure-User "demo_admin@trademind.local" "DemoAdmin123!" "Demo Admin" "admin"
$demoOp = Ensure-User "demo_operator@trademind.local" "DemoOperator123!" "Demo Operator" "operator"
$demoRo = Ensure-User "demo_readonly@trademind.local" "DemoReadonly123!" "Demo Readonly" "readonly"

if ($authorizedShop -and $demoOp.id) {
    $items = @(@{ storeId = $authorizedShop.id; permissionScope = "operate" })
    Invoke-Api -Method Put -Url "$ApiV1/admin/users/$($demoOp.id)/store-permissions" -Token $token `
        -Body (@{ items = $items } | ConvertTo-Json -Depth 4) | Out-Null
}
if ($authorizedShop -and $demoRo.id) {
    $items = @(@{ storeId = $authorizedShop.id; permissionScope = "view" })
    Invoke-Api -Method Put -Url "$ApiV1/admin/users/$($demoRo.id)/store-permissions" -Token $token `
        -Body (@{ items = $items } | ConvertTo-Json -Depth 4) | Out-Null
}

Write-Host "F5 demo users seeded. Authorized shop:" $authorizedShop.shopName
Write-Host "Denied shop sample:" $deniedShop.shopName
