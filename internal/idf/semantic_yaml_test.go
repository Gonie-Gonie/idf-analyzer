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
		!strings.Contains(projection.Text, "zones:") ||
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
