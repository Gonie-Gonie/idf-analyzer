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

- Semantic YAML now exposes EnergyPlus compatibility adapter metadata, split ZoneList/SpaceList/ZoneGroup views, zone Spaces, richer output attachment resolution, and expanded HVAC loop metadata.
- Diagnose, Summary, HVAC, and semantic exports now expose source, confidence, and evidence metadata for inferred or analyzer-limited results.

## Changed

- HVAC analysis now treats Branch components as EP26-style 4-field groups by default and includes CondenserLoop, connector/branch rule diagnostics, and ZoneHVAC equipment sequence metadata.
- Summary and HVAC UI views now surface confidence/source badges, grouped diagnostic sources, HVAC component families, and a compact/full semantic YAML toggle.

## Fixed

- _None._
