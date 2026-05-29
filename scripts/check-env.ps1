$ErrorActionPreference = "Stop"

. "$PSScriptRoot\toolchain.ps1"

$paths = Use-RepoToolchain

Write-Host "Repo root: $($paths.RepoRoot)"
Write-Host "Runtime root: $($paths.RuntimeRoot)"

$missing = $false
if (Test-Path -LiteralPath $paths.GoExe) {
    Write-Host "[ok] repo-local Go -> $($paths.GoExe)"
    & $paths.GoExe version
} else {
    Write-Host "[missing] repo-local Go -> $($paths.GoExe)"
    $missing = $true
}

if (Test-Path -LiteralPath $paths.WailsExe) {
    Write-Host "[ok] repo-local Wails -> $($paths.WailsExe)"
    & $paths.WailsExe version
} else {
    Write-Host "[missing] repo-local Wails -> $($paths.WailsExe)"
    $missing = $true
}

if ($missing) {
    throw "Repo-local runtime is incomplete. Run: powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup.ps1"
}
