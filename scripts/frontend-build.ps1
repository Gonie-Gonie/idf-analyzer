$ErrorActionPreference = "Stop"

$dist = Join-Path $PSScriptRoot "..\frontend\dist"
$index = Join-Path $dist "index.html"
$guide = Join-Path $dist "guide.html"

if (-not (Test-Path $index)) {
    throw "Missing frontend/dist/index.html"
}

if (-not (Test-Path $guide)) {
    throw "Missing frontend/dist/guide.html"
}

Write-Host "Static frontend is ready."
