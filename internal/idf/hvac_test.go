package idf

import "testing"

func TestAnalyzeHVACBuildsLoopAndZoneRelations(t *testing.T) {
	doc := Document{Objects: []Object{
		{Index: 0, Type: "AirLoopHVAC", Fields: []Field{
			{Value: "Main Air Loop"},
			{Value: ""},
			{Value: ""},
			{Value: "Autosize"},
			{Value: "Air Branches"},
			{Value: ""},
			{Value: "Air Supply Inlet"},
			{Value: "Air Demand Outlet"},
			{Value: "Air Demand Inlet"},
			{Value: "Air Supply Outlet"},
		}},
		{Index: 1, Type: "BranchList", Fields: []Field{
			{Value: "Air Branches"},
			{Value: "Main Air Branch"},
		}},
		{Index: 2, Type: "Branch", Fields: []Field{
			{Value: "Main Air Branch"},
			{Value: ""},
			{Value: "Fan:ConstantVolume"},
			{Value: "Supply Fan"},
			{Value: "Air Supply Inlet"},
			{Value: "Fan Outlet"},
			{Value: "Passive"},
			{Value: "Coil:Cooling:Water"},
			{Value: "Cooling Coil"},
			{Value: "Fan Outlet"},
			{Value: "Air Supply Outlet"},
			{Value: "Passive"},
		}},
		{Index: 3, Type: "Fan:ConstantVolume", Fields: []Field{
			{Value: "Supply Fan", Comment: "Name"},
			{Value: "Air Supply Inlet", Comment: "Air Inlet Node Name"},
			{Value: "Fan Outlet", Comment: "Air Outlet Node Name"},
		}},
		{Index: 4, Type: "Coil:Cooling:Water", Fields: []Field{
			{Value: "Cooling Coil", Comment: "Name"},
			{Value: "Fan Outlet", Comment: "Air Inlet Node Name"},
			{Value: "Air Supply Outlet", Comment: "Air Outlet Node Name"},
			{Value: "CHW Supply", Comment: "Water Inlet Node Name"},
			{Value: "CHW Return", Comment: "Water Outlet Node Name"},
		}},
		{Index: 5, Type: "PlantLoop", Fields: []Field{
			{Value: "Chilled Water Loop"},
			{Value: "Water"},
			{Value: ""},
			{Value: ""},
			{Value: "CHW Setpoint"},
			{Value: "15"},
			{Value: "5"},
			{Value: "Autosize"},
			{Value: "0"},
			{Value: "Autosize"},
			{Value: "Plant Supply Inlet"},
			{Value: "Plant Supply Outlet"},
			{Value: ""},
			{Value: ""},
			{Value: "CHW Supply"},
			{Value: "CHW Return"},
			{Value: "CHW Demand Branches"},
			{Value: ""},
		}},
		{Index: 6, Type: "BranchList", Fields: []Field{
			{Value: "CHW Demand Branches"},
			{Value: "CHW Coil Branch"},
		}},
		{Index: 7, Type: "Branch", Fields: []Field{
			{Value: "CHW Coil Branch"},
			{Value: ""},
			{Value: "Coil:Cooling:Water"},
			{Value: "Cooling Coil"},
			{Value: "CHW Supply"},
			{Value: "CHW Return"},
			{Value: "Passive"},
		}},
		{Index: 8, Type: "Zone", Fields: []Field{{Value: "Office"}}},
		{Index: 9, Type: "NodeList", Fields: []Field{
			{Value: "Office Inlets"},
			{Value: "Office Supply Inlet"},
		}},
		{Index: 10, Type: "ZoneHVAC:EquipmentConnections", Fields: []Field{
			{Value: "Office"},
			{Value: "Office Equipment"},
			{Value: "Office Inlets"},
			{Value: ""},
			{Value: "Office Zone Air Node"},
			{Value: "Office Return Node"},
		}},
		{Index: 11, Type: "ZoneHVAC:EquipmentList", Fields: []Field{
			{Value: "Office Equipment"},
			{Value: "AirTerminal:SingleDuct:ConstantVolume:NoReheat"},
			{Value: "Office Terminal"},
			{Value: "1"},
			{Value: "1"},
		}},
		{Index: 12, Type: "AirTerminal:SingleDuct:ConstantVolume:NoReheat", Fields: []Field{
			{Value: "Office Terminal", Comment: "Name"},
			{Value: "Air Demand Inlet", Comment: "Air Inlet Node Name"},
			{Value: "Office Supply Inlet", Comment: "Air Outlet Node Name"},
		}},
	}}

	report := AnalyzeHVAC(doc)
	if report.AirLoopCount != 1 || report.PlantLoopCount != 1 {
		t.Fatalf("loop counts = air %d plant %d, want 1/1", report.AirLoopCount, report.PlantLoopCount)
	}
	airLoop := findHVACTestingLoop(report, "Main Air Loop")
	if airLoop == nil {
		t.Fatalf("Main Air Loop not found in %#v", report.Loops)
	}
	if got := len(airLoop.SupplySide.Branches[0].Components); got != 2 {
		t.Fatalf("air branch components = %d, want 2", got)
	}
	if len(airLoop.RelatedLoops) == 0 {
		t.Fatalf("expected cross-loop relation for shared cooling coil")
	}
	if len(report.ZoneRelations) != 1 {
		t.Fatalf("zone relation count = %d, want 1", len(report.ZoneRelations))
	}
	relation := report.ZoneRelations[0]
	if !stringSliceContainsFold(relation.AirLoopNames, "Main Air Loop") {
		t.Fatalf("air loop names = %#v, want Main Air Loop", relation.AirLoopNames)
	}
	if !stringSliceContainsFold(relation.PlantLoopNames, "Chilled Water Loop") {
		t.Fatalf("plant loop names = %#v, want Chilled Water Loop", relation.PlantLoopNames)
	}
	if hasHVACWarningCode(report.Warnings, "water_coil_missing_plant_loop") {
		t.Fatalf("unexpected water coil warning: %#v", report.Warnings)
	}
}

func TestAnalyzeHVACReportsMissingBranch(t *testing.T) {
	doc := Document{Objects: []Object{
		{Index: 0, Type: "AirLoopHVAC", Fields: []Field{
			{Value: "Main Air Loop"},
			{Value: ""},
			{Value: ""},
			{Value: "Autosize"},
			{Value: "Air Branches"},
		}},
		{Index: 1, Type: "BranchList", Fields: []Field{
			{Value: "Air Branches"},
			{Value: "Missing Branch"},
		}},
	}}

	report := AnalyzeHVAC(doc)
	if !hasHVACWarningCode(report.Warnings, "missing_branch") {
		t.Fatalf("warnings = %#v, want missing_branch", report.Warnings)
	}
}

func findHVACTestingLoop(report HVACReport, name string) *HVACLoop {
	for index := range report.Loops {
		if report.Loops[index].Name == name {
			return &report.Loops[index]
		}
	}
	return nil
}

func hasHVACWarningCode(warnings []HVACWarning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
