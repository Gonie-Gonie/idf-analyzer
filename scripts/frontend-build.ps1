$ErrorActionPreference = "Stop"

$dist = Join-Path $PSScriptRoot "..\frontend\dist"
$index = Join-Path $dist "index.html"

if (-not (Test-Path $index)) {
    throw "Missing frontend/dist/index.html"
}

Write-Host "Static frontend is ready."
