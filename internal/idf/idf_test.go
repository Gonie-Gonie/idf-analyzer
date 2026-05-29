package idf

import "testing"

const sampleIDF = `
Version,
  24.1;                    !- Version Identifier

ScheduleTypeLimits,
  Fraction;                 !- Name

Schedule:Compact,
  AlwaysOn,                 !- Name
  Fraction,                 !- Schedule Type Limits Name
  Through: 12/31,           !- Field 1
  For: AllDays,             !- Field 2
  Until: 24:00,             !- Field 3
  1;                        !- Field 4

Schedule:Compact,
  UnusedSchedule,           !- Name
  Fraction,                 !- Schedule Type Limits Name
  Through: 12/31,           !- Field 1
  For: AllDays,             !- Field 2
  Until: 24:00,             !- Field 3
  0;                        !- Field 4

Zone,
  Office;                   !- Name

Lights,
  Office Lights,            !- Name
  Office,                   !- Zone or ZoneList Name
  AlwaysOn,                 !- Schedule Name
  LightingLevel,            !- Design Level Calculation Method
  500;                      !- Lighting Level

Fan:ConstantVolume,
  Supply Fan,               !- Name
  AlwaysOn,                 !- Availability Schedule Name
  0.7,                      !- Fan Total Efficiency
  500,                      !- Pressure Rise
  1.0,                      !- Maximum Flow Rate
  0.9,                      !- Motor Efficiency
  1.0,                      !- Motor In Airstream Fraction
  Air Inlet Node,           !- Air Inlet Node Name
  Air Outlet Node;          !- Air Outlet Node Name
`

func TestParseKeepsFieldComments(t *testing.T) {
	doc, err := Parse(sampleIDF)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(doc.Objects) != 7 {
		t.Fatalf("object count = %d, want 7", len(doc.Objects))
	}
	if got := doc.Objects[2].Fields[0].Value; got != "AlwaysOn" {
		t.Fatalf("schedule name = %q, want AlwaysOn", got)
	}
	if got := doc.Objects[2].Fields[0].Comment; got != "Name" {
		t.Fatalf("schedule comment = %q, want Name", got)
	}
}

func TestAnalyzeFindsSchedulesConnectionsAndUnusedObjects(t *testing.T) {
	doc, err := Parse(sampleIDF)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	report := Analyze(doc)

	if report.ObjectCount != 7 {
		t.Fatalf("object count = %d, want 7", report.ObjectCount)
	}
	if len(report.Schedules) != 2 {
		t.Fatalf("schedule count = %d, want 2", len(report.Schedules))
	}
	if len(report.HVACConnections) != 1 {
		t.Fatalf("connection count = %d, want 1", len(report.HVACConnections))
	}
	connection := report.HVACConnections[0]
	if connection.FromNode != "Air Inlet Node" || connection.ToNode != "Air Outlet Node" {
		t.Fatalf("connection = %#v, want inlet to outlet", connection)
	}
	if len(report.UnusedObjects) != 1 {
		t.Fatalf("unused count = %d, want 1: %#v", len(report.UnusedObjects), report.UnusedObjects)
	}
	if report.UnusedObjects[0].Name != "UnusedSchedule" {
		t.Fatalf("unused name = %q, want UnusedSchedule", report.UnusedObjects[0].Name)
	}
}

func TestUpdateFieldAndRemoveUnusedObjects(t *testing.T) {
	doc, err := Parse(sampleIDF)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	updated, err := UpdateField(doc, 4, 0, "Core Office")
	if err != nil {
		t.Fatalf("UpdateField() error = %v", err)
	}
	if got := updated.Objects[4].Fields[0].Value; got != "Core Office" {
		t.Fatalf("updated zone = %q, want Core Office", got)
	}
	if got := doc.Objects[4].Fields[0].Value; got != "Office" {
		t.Fatalf("original zone mutated = %q", got)
	}

	trimmed, removed := RemoveUnusedObjects(doc)
	if len(removed) != 1 {
		t.Fatalf("removed count = %d, want 1", len(removed))
	}
	if len(trimmed.Objects) != 6 {
		t.Fatalf("trimmed count = %d, want 6", len(trimmed.Objects))
	}
	roundTrip, err := Parse(trimmed.String())
	if err != nil {
		t.Fatalf("round trip parse error = %v", err)
	}
	if len(roundTrip.Objects) != 6 {
		t.Fatalf("round trip count = %d, want 6", len(roundTrip.Objects))
	}
}
