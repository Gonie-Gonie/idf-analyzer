package idf

import "testing"

func TestFieldCatalogFindsFieldsWithoutComments(t *testing.T) {
	obj := Object{
		Type: "AirLoopHVAC",
		Fields: []Field{
			{Value: "Main Air Loop"},
			{Value: "Controllers"},
			{Value: "Availability"},
			{Value: "autosize"},
			{Value: "Main Branches"},
			{Value: "Main Connectors"},
			{Value: "Supply Inlet"},
		},
	}

	if got := fieldValueByCatalogName(obj, "Branch List Name"); got != "Main Branches" {
		t.Fatalf("Branch List Name = %q, want Main Branches", got)
	}
	if got := catalogFieldRole(obj, 6); got != fieldRoleNodeRef {
		t.Fatalf("field role = %q, want node ref", got)
	}
}

func TestFieldCatalogFallsBackToComments(t *testing.T) {
	obj := Object{
		Type: "Custom:Object",
		Fields: []Field{
			{Value: "Obj", Comment: "Name"},
			{Value: "Node A", Comment: "Air Inlet Node Name"},
		},
	}

	if got := fieldValueByCatalogName(obj, "Air Inlet Node Name"); got != "Node A" {
		t.Fatalf("comment fallback = %q, want Node A", got)
	}
}

func TestSuggestFieldValuesUsesCatalogAndDocument(t *testing.T) {
	doc := Document{Objects: []Object{
		{
			Index: 0,
			Type:  "BranchList",
			Fields: []Field{
				{Value: "Main Branches"},
			},
		},
		{
			Index: 1,
			Type:  "AirLoopHVAC",
			Fields: []Field{
				{Value: "Main Air Loop"},
				{Value: ""},
				{Value: ""},
				{Value: "Autosize"},
				{Value: ""},
			},
		},
	}}

	suggestions := SuggestFieldValues(doc, 1, 4)
	if !hasSuggestionValue(suggestions, "Main Branches") {
		t.Fatalf("suggestions = %#v, want Main Branches", suggestions)
	}
}

func TestFieldCatalogDiagnosticsValidatesChoicesAndNumbers(t *testing.T) {
	doc := Document{Objects: []Object{
		{
			Index: 0,
			Type:  "PlantLoop",
			Fields: []Field{
				{Value: "Hot Water Loop"},
				{Value: "Coffee"},
				{Value: ""},
				{Value: ""},
				{Value: "Loop Setpoint"},
				{Value: "warm"},
			},
		},
	}}

	diagnostics := fieldCatalogDiagnostics(doc)
	if !hasDiagnosticCode(diagnostics, "invalid_choice") {
		t.Fatalf("diagnostics = %#v, want invalid_choice", diagnostics)
	}
	if !hasDiagnosticCode(diagnostics, "invalid_number") {
		t.Fatalf("diagnostics = %#v, want invalid_number", diagnostics)
	}
}

func hasSuggestionValue(suggestions []FieldValueSuggestion, value string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Value == value {
			return true
		}
	}
	return false
}

func hasDiagnosticCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
