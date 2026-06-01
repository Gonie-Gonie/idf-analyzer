package idf

import "testing"

func TestPreviewApplyHVACAcceptsSafeCapacityEdit(t *testing.T) {
	doc := Document{Objects: []Object{
		{Index: 0, Type: "Coil:Cooling:Water", Fields: []Field{
			{Value: "Cooling Coil", Comment: "Name"},
			{Value: "Autosize", Comment: "Design Water Flow Rate"},
			{Value: "Autosize", Comment: "Design Total Cooling Capacity"},
		}},
	}}

	preview := PreviewApplyHVAC(doc, HVACApplyRequest{Changes: []HVACFieldEditRequest{
		{ObjectIndex: 0, FieldIndex: 2, Value: "12000"},
	}})
	if !preview.CanApply {
		t.Fatalf("preview.CanApply = false, warnings = %#v", preview.Warnings)
	}
	if len(preview.Changes) != 1 || preview.Changes[0].After != "12000" {
		t.Fatalf("preview changes = %#v, want one 12000 change", preview.Changes)
	}

	updated, applied := ApplyHVAC(doc, HVACApplyRequest{Changes: []HVACFieldEditRequest{
		{ObjectIndex: 0, FieldIndex: 2, Value: "12000"},
	}})
	if !applied.CanApply {
		t.Fatalf("applied.CanApply = false, warnings = %#v", applied.Warnings)
	}
	if got := updated.Objects[0].Fields[2].Value; got != "12000" {
		t.Fatalf("updated capacity = %q, want 12000", got)
	}
}

func TestPreviewApplyHVACRejectsUnsafeField(t *testing.T) {
	doc := Document{Objects: []Object{
		{Index: 0, Type: "Building", Fields: []Field{
			{Value: "Building", Comment: "Name"},
		}},
	}}

	preview := PreviewApplyHVAC(doc, HVACApplyRequest{Changes: []HVACFieldEditRequest{
		{ObjectIndex: 0, FieldIndex: 0, Value: "New Building"},
	}})
	if preview.CanApply {
		t.Fatalf("preview.CanApply = true, want false")
	}
	if !hasHVACWarningCode(preview.Warnings, "unsafe_hvac_field") {
		t.Fatalf("warnings = %#v, want unsafe_hvac_field", preview.Warnings)
	}
}

func TestPreviewApplyHVACRejectsInvalidNumber(t *testing.T) {
	doc := Document{Objects: []Object{
		{Index: 0, Type: "Fan:ConstantVolume", Fields: []Field{
			{Value: "Supply Fan", Comment: "Name"},
			{Value: "Autosize", Comment: "Maximum Flow Rate"},
		}},
	}}

	preview := PreviewApplyHVAC(doc, HVACApplyRequest{Changes: []HVACFieldEditRequest{
		{ObjectIndex: 0, FieldIndex: 1, Value: "fast"},
	}})
	if preview.CanApply {
		t.Fatalf("preview.CanApply = true, want false")
	}
	if !hasHVACWarningCode(preview.Warnings, "invalid_hvac_number") {
		t.Fatalf("warnings = %#v, want invalid_hvac_number", preview.Warnings)
	}
}
