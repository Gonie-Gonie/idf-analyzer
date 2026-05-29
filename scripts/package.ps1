$ErrorActionPreference = "Stop"

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    throw "Wails CLI is required for packaging. Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

wails build
