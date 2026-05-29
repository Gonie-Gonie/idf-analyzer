# IDF Analyzer

Lightweight desktop tooling for EnergyPlus IDF files, built with Go and Wails using a static HTML/CSS/JS frontend.

## Current Scope

- Parse IDF objects and field comments.
- Summarize object types, schedules, zones, unused named objects, and simple HVAC node connections.
- Edit field values and remove unused named objects through the Go API.
- Run the frontend without a Node/npm build chain.

## Requirements

- Go 1.24 or newer.
- Wails v2 CLI for packaged builds.
- Platform webview runtime required by Wails.

Install Wails CLI when packaging is needed:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## Commands

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check-env.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package.ps1
```

`scripts/run.ps1` uses `go run .` and does not require Node/npm. `scripts/package.ps1` uses Wails.

## Project Layout

- `internal/idf`: IDF parsing, analysis, and editing core.
- `frontend/dist`: tracked static frontend assets.
- `app.go`: Wails-bound application API.
- `scripts`: environment checks and repeatable commands.
