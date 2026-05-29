# Codex Working Notes

- Primary stack: Go plus Wails v2 with static HTML/CSS/JS.
- Runtime policy: install Go and Wails into ignored `.runtime/` via `scripts/setup.ps1`; do not rely on global Go/Wails for project verification.
- Core behavior should live under `internal/idf` so parser, analyzer, and editor logic stays easy to test.
- UI should stay dense and work-focused: editor, object tables, schedules, zones, and HVAC connection views first.
- Avoid adding npm unless the frontend genuinely needs bundling or a component framework.
- Before committing, run `scripts/verify.ps1` so tests and `wails build` pass with the repo-local runtime.
- Treat unused-object deletion conservatively and keep parser round trips covered by tests.
