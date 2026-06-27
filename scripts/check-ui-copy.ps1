# Phase R1.2-Auto — UI copywriting static scan (internal codes + mixed English).
# Usage: .\scripts\check-ui-copy.ps1 [-OutFile docs/COPYWRITING_AUDIT.auto.md]

param(
    [string]$OutFile = "docs/COPYWRITING_AUDIT.auto.md"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    node scripts/check-ui-copy.mjs --strict --report $OutFile
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
