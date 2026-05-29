param(
    [string]$GoVersion = "1.24.5",
    [string]$WailsVersion = "v2.12.0",
    [switch]$Force,
    [switch]$SkipGitHook
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

. "$PSScriptRoot\toolchain.ps1"

function Get-GoArchiveArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString()
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture for repo-local Go runtime: $arch" }
    }
}

function Assert-PathUnderRoot {
    param(
        [string]$Root,
        [string]$Path
    )

    $rootFull = [System.IO.Path]::GetFullPath((Resolve-Path -LiteralPath $Root).Path)
    $targetFull = [System.IO.Path]::GetFullPath($Path)
    $trimChars = [char[]]@('\', '/')
    $rootPrefix = $rootFull.TrimEnd($trimChars) + [System.IO.Path]::DirectorySeparatorChar
    $targetPrefix = $targetFull.TrimEnd($trimChars) + [System.IO.Path]::DirectorySeparatorChar

    if (-not $targetPrefix.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to modify path outside runtime root: $targetFull"
    }
}

function Remove-RuntimeTree {
    param([string]$Path)

    $paths = Get-RepoToolchain
    if (Test-Path -LiteralPath $Path) {
        Assert-PathUnderRoot -Root $paths.RuntimeRoot -Path $Path
        Remove-Item -LiteralPath $Path -Recurse -Force
    }
}

function Get-InstalledGoVersion {
    param([string]$GoExe)

    if (-not (Test-Path -LiteralPath $GoExe)) {
        return ""
    }

    return (& $GoExe version 2>$null | Out-String).Trim()
}

function Install-GoRuntime {
    param(
        [object]$Paths,
        [string]$Version,
        [switch]$ForceInstall
    )

    $installed = Get-InstalledGoVersion -GoExe $Paths.GoExe
    if (-not $ForceInstall -and $installed -match "go$([regex]::Escape($Version))") {
        Write-Host "[ok] Go runtime already installed: $installed"
        return
    }

    Write-Host "[setup] Installing Go $Version under $($Paths.RuntimeRoot)"
    Remove-RuntimeTree -Path $Paths.GoRoot

    $arch = Get-GoArchiveArch
    $archiveName = "go$Version.windows-$arch.zip"
    $archivePath = Join-Path $Paths.CacheDir $archiveName
    $url = "https://go.dev/dl/$archiveName"

    if (-not (Test-Path -LiteralPath $archivePath)) {
        Write-Host "[download] $url"
        Invoke-WebRequest -Uri $url -OutFile $archivePath
    }

    $extractDir = Join-Path $Paths.CacheDir "go-$Version-$arch"
    Remove-RuntimeTree -Path $extractDir
    New-Item -ItemType Directory -Path $extractDir | Out-Null
    Expand-Archive -Path $archivePath -DestinationPath $extractDir

    $extractedGo = Join-Path $extractDir "go"
    if (-not (Test-Path -LiteralPath $extractedGo)) {
        throw "Downloaded Go archive did not contain a go directory."
    }

    Move-Item -LiteralPath $extractedGo -Destination $Paths.GoRoot
    Remove-RuntimeTree -Path $extractDir
}

function Get-WailsVersionText {
    param([string]$WailsExe)

    if (-not (Test-Path -LiteralPath $WailsExe)) {
        return ""
    }

    return (& $WailsExe version 2>$null | Out-String).Trim()
}

function Install-WailsCLI {
    param(
        [object]$Paths,
        [string]$Version,
        [switch]$ForceInstall
    )

    $installed = Get-WailsVersionText -WailsExe $Paths.WailsExe
    $versionNeedle = $Version.TrimStart("v")
    if (-not $ForceInstall -and $installed -match [regex]::Escape($versionNeedle)) {
        Write-Host "[ok] Wails CLI already installed: $installed"
        return
    }

    Write-Host "[setup] Installing Wails CLI $Version into $($Paths.BinDir)"
    & $Paths.GoExe install "github.com/wailsapp/wails/v2/cmd/wails@$Version"
}

$paths = Get-RepoToolchain
foreach ($dir in @($paths.RuntimeRoot, $paths.BinDir, $paths.CacheDir)) {
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
}

Install-GoRuntime -Paths $paths -Version $GoVersion -ForceInstall:$Force
$paths = Use-RepoToolchain -RequireGo
Install-WailsCLI -Paths $paths -Version $WailsVersion -ForceInstall:$Force

Write-Host "[ok] Toolchain ready"
& $paths.GoExe version
& $paths.WailsExe version

if (-not $SkipGitHook) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "install-hooks.ps1")
}
