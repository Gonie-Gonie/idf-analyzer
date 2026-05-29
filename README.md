# IDF Analyzer

Lightweight desktop tooling for EnergyPlus IDF files, built with Go and Wails using a static HTML/CSS/JS frontend.

## Current Scope

- Parse IDF objects and field comments.
- Summarize object types, schedules, zones, unused named objects, and simple HVAC node connections.
- Edit field values and remove unused named objects through the Go API.
- Run the frontend without a Node/npm build chain.

## Requirements

- PowerShell.
- Internet access for the first setup.
- Platform webview runtime required by Wails.

The Go runtime and Wails CLI are installed into `.runtime/` by setup. That directory is local to each clone and is ignored by git.

Default setup versions:

- Go 1.24.5
- Wails CLI v2.12.0

## Commands

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check-env.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\package.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\verify.ps1
```

`scripts/setup.ps1` installs the repo-local runtime and a pre-commit hook. The hook runs `scripts/verify.ps1`, which performs whitespace checks, `go test ./...`, and `wails build` using `.runtime/`.

Build artifacts and downloaded runtimes stay ignored by git.

## Project Layout

- `internal/idf`: IDF parsing, analysis, and editing core.
- `frontend/dist`: tracked static frontend assets.
- `app.go`: Wails-bound application API.
- `scripts`: repo-local runtime setup, checks, and repeatable commands.
- `.runtime`: ignored local Go/Wails runtime and caches created by setup.
