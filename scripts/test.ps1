$ErrorActionPreference = "Stop"

. "$PSScriptRoot\toolchain.ps1"

$paths = Use-RepoToolchain -RequireGo
& $paths.GoExe test ./...
