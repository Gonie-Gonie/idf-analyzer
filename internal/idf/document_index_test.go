package idf

import "testing"

func TestDocumentIndexProvidesTypeAndNameLookups(t *testing.T) {
	doc, err := Parse(`
Version,
  24.1;

Zone,
  Office;

Schedule:Constant,
  Always On,
  ,
  1;
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	index := NewDocumentIndex(doc)
	if got := len(index.ObjectsOfType("Zone")); got != 1 {
		t.Fatalf("zone lookup count = %d, want 1", got)
	}
	if got := len(index.ObjectsNamed("office")); got != 1 {
		t.Fatalf("name lookup count = %d, want 1", got)
	}
	if _, ok := index.ObjectByTypeName("Schedule:Constant", "Always On"); !ok {
		t.Fatalf("type/name lookup did not find Schedule:Constant Always On")
	}
	if got := len(index.Schedules); got != 1 {
		t.Fatalf("schedule index count = %d, want 1", got)
	}
}
