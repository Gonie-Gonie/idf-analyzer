$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
}

function Get-RepoToolchain {
    $repoRoot = Get-RepoRoot
    $runtimeRoot = Join-Path $repoRoot ".runtime"
    $goRoot = Join-Path $runtimeRoot "go"
    $binDir = Join-Path $runtimeRoot "bin"

    return [pscustomobject]@{
        RepoRoot = $repoRoot
        RuntimeRoot = $runtimeRoot
        GoRoot = $goRoot
        GoBinDir = Join-Path $goRoot "bin"
        GoExe = Join-Path $goRoot "bin\go.exe"
        BinDir = $binDir
        WailsExe = Join-Path $binDir "wails.exe"
        GoPath = Join-Path $runtimeRoot "gopath"
        GoModCache = Join-Path $runtimeRoot "gomodcache"
        GoCache = Join-Path $runtimeRoot "gocache"
        CacheDir = Join-Path $runtimeRoot "cache"
    }
}

function Use-RepoToolchain {
    param(
        [switch]$RequireGo,
        [switch]$RequireWails
    )

    $paths = Get-RepoToolchain
    foreach ($dir in @($paths.RuntimeRoot, $paths.BinDir, $paths.GoPath, $paths.GoModCache, $paths.GoCache, $paths.CacheDir)) {
        if (-not (Test-Path -LiteralPath $dir)) {
            New-Item -ItemType Directory -Path $dir | Out-Null
        }
    }

    $env:GOROOT = $paths.GoRoot
    $env:GOPATH = $paths.GoPath
    $env:GOBIN = $paths.BinDir
    $env:GOMODCACHE = $paths.GoModCache
    $env:GOCACHE = $paths.GoCache
    $env:GOTOOLCHAIN = "local"
    $env:PATH = "$($paths.BinDir);$($paths.GoBinDir);$env:PATH"

    if ($RequireGo -and -not (Test-Path -LiteralPath $paths.GoExe)) {
        throw "Repo-local Go runtime is missing. Run: powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup.ps1"
    }

    if ($RequireWails -and -not (Test-Path -LiteralPath $paths.WailsExe)) {
        throw "Repo-local Wails CLI is missing. Run: powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup.ps1"
    }

    return $paths
}
