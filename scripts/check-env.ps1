$ErrorActionPreference = "Stop"

function Test-Command($Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        Write-Host "[missing] $Name"
        return $false
    }

    Write-Host "[ok] $Name -> $($command.Source)"
    return $true
}

$hasGo = Test-Command "go"
$hasWails = Test-Command "wails"

if ($hasGo) {
    go version
}

if ($hasWails) {
    wails version
} else {
    Write-Host "Wails CLI is only required for packaged builds."
    Write-Host "Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

if (-not $hasGo) {
    throw "Go is required for tests and local runs."
}
