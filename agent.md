# Agent Notes

- Keep the committed project lightweight: repo-local runtime downloads live in ignored `.runtime/`.
- Use `scripts/setup.ps1` to prepare `.runtime/go`, `.runtime/bin/wails.exe`, and local Go caches per clone.
- Prefer static frontend assets until a build chain becomes clearly valuable.
- Every implementation pass should end with `scripts/verify.ps1`, then commit and push when the work is complete.
- Every commit should use the repo-local runtime and include a successful Wails build; setup installs a local pre-commit hook for this.
- Protect user work in the git tree. Do not revert unrelated changes.
- Favor small IDF-domain functions that can be tested without launching the desktop shell.
