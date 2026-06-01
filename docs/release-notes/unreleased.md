# Unreleased Release Notes

<!--
Add release-note entries under the section that best describes the change.
The release script infers bump size from these sections:
- Breaking Changes: major
- Added or Features: minor
- Fixed, Changed, Performance, Security, Documentation, or internal-only notes: patch
-->

## Breaking Changes

- _None._

## Added

- Profile Analysis for normalized internal-load, ventilation, outdoor-air, schedule pattern, matrix, graph, and apply-preview workflows.
- HVAC analysis views for loop diagrams, system-zone relation graphs, connection diagnostics, cross-loop navigation, node inspection, and output-variable guidance.
- User-configurable Analyze tab order, GUI language selection, keyboard shortcuts, view-location undo/redo, and definition/reference jumps in text, JSON, and table input views.
- Fullscreen-style in-app expansion for HVAC and Geometry views plus geometry selection aids and construction layer graphics.

## Changed

- Refined the main UI sizing, scroll containers, visual hierarchy, profile graph focus, HVAC selectors, and geometry detail panels for denser analysis workflows.
- Updated release automation to validate app version consistency across Wails metadata, bundled app info, and static HTML placeholders.

## Fixed

- Restored native scroll-axis behavior so vertical and horizontal scrolling stay visually predictable.
- Reduced release/version drift risk by updating static page version labels during release preparation.
- Reused translated status messages for input navigation and field updates.
