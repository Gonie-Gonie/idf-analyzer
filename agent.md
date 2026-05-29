# Agent Notes

- Keep the project lightweight: no repo-local Go, Node, or other runtime downloads.
- Prefer static frontend assets until a build chain becomes clearly valuable.
- Every implementation pass should end with tests, then commit and push when the work is complete.
- Protect user work in the git tree. Do not revert unrelated changes.
- Favor small IDF-domain functions that can be tested without launching the desktop shell.
