# Seed product drafts for list performance testing (Phase A1.1).
# Usage: .\scripts\seed-product-list-perf.ps1 -Count 1000 [-ApiBase http://127.0.0.1:8080]

param(
    [int]$Count = 500,
    [string]$ApiBase = "http://127.0.0.1:8080",
    [string]$Account = $env:ADMIN_BOOTSTRAP_EMAIL,
    [string]$Password = $env:ADMIN_BOOTSTRAP_PASSWORD
)

$ErrorActionPreference = "Stop"
$ApiV1 = "$ApiBase/api/v1"

$login = Invoke-RestMethod -Method Post -Uri "$ApiV1/auth/login" -ContentType "application/json" `
    -Body (@{ account = $Account; password = $Password } | ConvertTo-Json)
$token = $login.data.token
if (-not $token) { Write-Error "login failed"; exit 1 }
$headers = @{ Authorization = "Bearer $token"; Accept = "application/json" }

$sources = @("1688", "pinduoduo", "taobao_tmall", "custom", "manual")
$created = 0
$batch = 20
$sw = [System.Diagnostics.Stopwatch]::StartNew()

for ($i = 0; $i -lt $Count; $i++) {
    $src = $sources[$i % $sources.Length]
    $body = @{
        source = $src
        title = "Perf seed #$i $src"
        description = "Performance seed product $i with enough description text for operation progress filters."
        currency = "CNY"
        status = "draft"
        skus = @(@{
            skuName = "Default"
            price = 9.9 + ($i % 50)
            stock = 100
        })
    } | ConvertTo-Json -Depth 5

    Invoke-RestMethod -Method Post -Uri "$ApiV1/products" -Headers $headers -ContentType "application/json" -Body $body | Out-Null
    $created++
    if ($created % $batch -eq 0) {
        Write-Host "Created $created / $Count ($([math]::Round($sw.Elapsed.TotalSeconds, 1))s)"
    }
}

Write-Host "Done. Created $created products in $([math]::Round($sw.Elapsed.TotalSeconds, 1))s"
