# Codex Working Notes

- Primary stack: Go plus Wails v2 with static HTML/CSS/JS.
- Core behavior should live under `internal/idf` so parser, analyzer, and editor logic stays easy to test.
- UI should stay dense and work-focused: editor, object tables, schedules, zones, and HVAC connection views first.
- Avoid adding npm unless the frontend genuinely needs bundling or a component framework.
- Treat unused-object deletion conservatively and keep parser round trips covered by tests.
