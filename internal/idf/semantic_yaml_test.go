package idf

import (
	"strings"
	"testing"
)

func TestBuildSemanticYAMLProjectionPreservesObjectMetadata(t *testing.T) {
	doc, err := Parse(`
Version,
  26.1;

Zone,
  Office;

People,
  Office People,
  Office,
  AlwaysOn,
  People,
  3;
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{EnergyPlusVersion: "26.1", SourceFormat: "idf"})
	if projection.Schema != semanticYAMLSchema {
		t.Fatalf("schema = %q", projection.Schema)
	}
	if projection.ObjectCount != 3 {
		t.Fatalf("object count = %d, want 3", projection.ObjectCount)
	}
	if !strings.Contains(projection.Text, "semantic_energyplus_model:") ||
		!strings.Contains(projection.Text, "schema: eplus-semantic/0.2") ||
		!strings.Contains(projection.Text, "zones:") ||
		!strings.Contains(projection.Text, "loads:") ||
		!strings.Contains(projection.Text, "people:") ||
		!strings.Contains(projection.Text, "source_preservation:") {
		t.Fatalf("projection text missing expected sections:\n%s", projection.Text)
	}

	foundEditableName := false
	for _, line := range projection.Lines {
		if line.Editable && line.ObjectIndex != nil && *line.ObjectIndex == 1 && line.FieldIndex != nil && *line.FieldIndex == 0 && line.Value == "Office" {
			foundEditableName = true
		}
	}
	if !foundEditableName {
		t.Fatalf("editable Zone name line not found in %#v", projection.Lines)
	}
}

func TestSemanticYAMLGroupsZoneLoadsAndOutputs(t *testing.T) {
	doc, err := Parse(`
Zone,
  Office;

People,
  Office People,
  Office,
  AlwaysOn,
  People,
  3;

Output:Variable,
  Office,
  Zone Mean Air Temperature,
  Hourly;
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{})
	for _, expected := range []string{
		"zones:",
		"- name: Office",
		"loads:",
		"people:",
		"schedule: AlwaysOn",
		"level: 3 persons",
		"outputs:",
		"- \"[Hourly] Zone Mean Air Temperature\"",
	} {
		if !strings.Contains(projection.Text, expected) {
			t.Fatalf("semantic YAML missing %q:\n%s", expected, projection.Text)
		}
	}
}

func TestSemanticYAMLShowsCompactScheduleAsRules(t *testing.T) {
	doc, err := Parse(`
Schedule:Compact,
  OfficeSched,
  Fraction,
  Through: 12/31,
  For: Weekdays,
  Until: 08:00,
  0,
  Until: 18:00,
  1,
  Until: 24:00,
  0;
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{})
	for _, expected := range []string{
		"schedules:",
		"- name: OfficeSched",
		"class: \"Schedule:Compact\"",
		"type_limits: Fraction",
		"rules:",
		"- through: 12/31",
		"for: Weekdays",
		"- time: \"08:00\"",
		"value: 0",
		"- time: \"18:00\"",
		"value: 1",
	} {
		if !strings.Contains(projection.Text, expected) {
			t.Fatalf("semantic YAML missing %q:\n%s", expected, projection.Text)
		}
	}

	foundEditableUntil := false
	for _, line := range projection.Lines {
		if line.Editable && line.ObjectIndex != nil && *line.ObjectIndex == 0 && line.FieldIndex != nil && *line.FieldIndex == 6 && line.Value == "18:00" {
			foundEditableUntil = true
		}
	}
	if !foundEditableUntil {
		t.Fatalf("editable compact schedule time line not found in %#v", projection.Lines)
	}
}

func TestSemanticYAMLShowsSurfaceVerticesAsZoneGeometry(t *testing.T) {
	doc, err := Parse(`
Zone,
  Office;

BuildingSurface:Detailed,
  Office Wall,              !- Name
  Wall,                     !- Surface Type
  ExtWall,                  !- Construction Name
  Office,                   !- Zone Name
  Outdoors,                 !- Outside Boundary Condition
  ,                         !- Outside Boundary Condition Object
  SunExposed,               !- Sun Exposure
  WindExposed,              !- Wind Exposure
  0.5,                      !- View Factor to Ground
  4,                        !- Number of Vertices
  0,0,3,                    !- Vertex 1
  4,0,3,                    !- Vertex 2
  4,0,0,                    !- Vertex 3
  0,0,0;                    !- Vertex 4
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{})
	for _, expected := range []string{
		"geometry:",
		"surfaces:",
		"- name: Office Wall",
		"type: Wall",
		"construction: ExtWall",
		"vertices: [[0,0,3], [4,0,3], [4,0,0], [0,0,0]]",
	} {
		if !strings.Contains(projection.Text, expected) {
			t.Fatalf("semantic YAML missing %q:\n%s", expected, projection.Text)
		}
	}
}

func TestSemanticYAMLPreservesUnmappedObjectsInMiscellaneous(t *testing.T) {
	doc, err := Parse(`
Zone,
  Office;

Parametric:SetValueForRun,
  Some Value,
  3.14;
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{})
	for _, expected := range []string{
		"miscellaneous:",
		"other:",
		"class: \"Parametric:SetValueForRun\"",
		"reason: unmapped_object_type",
		"export_policy: preserve_exactly",
	} {
		if !strings.Contains(projection.Text, expected) {
			t.Fatalf("semantic YAML missing %q:\n%s", expected, projection.Text)
		}
	}
}

func TestSemanticDuplicateNameFixesRenameLaterDuplicates(t *testing.T) {
	doc, err := Parse(`
Zone,
  Office;

Zone,
  Office;

Zone,
  Office 2;
`)
	if err != nil {
		t.Fatal(err)
	}

	projection := BuildSemanticYAMLProjection(doc, SemanticYAMLMetadata{})
	if len(projection.DuplicateGroups) != 1 {
		t.Fatalf("duplicate groups = %#v", projection.DuplicateGroups)
	}
	if !strings.Contains(projection.Text, "source_name_conflicts:") {
		t.Fatalf("semantic YAML should report name conflicts separately:\n%s", projection.Text)
	}

	updated, fixes := ApplySemanticDuplicateNameFixes(doc)
	if len(fixes) != 1 {
		t.Fatalf("fixes = %#v", fixes)
	}
	if fixes[0].Before != "Office" || fixes[0].After != "Office 3" {
		t.Fatalf("fix = %#v, want Office -> Office 3", fixes[0])
	}
	if objectName(updated.Objects[1]) != "Office 3" {
		t.Fatalf("updated name = %q", objectName(updated.Objects[1]))
	}
}
