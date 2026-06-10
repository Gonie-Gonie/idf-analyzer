package idf

import "testing"

func TestDiagnosticsClassifyRunPeriodMissingForDesignDay(t *testing.T) {
	doc, err := Parse(`
Version, 24.1;
Building, Test;
Timestep, 4;
SimulationControl, Yes, Yes, No, No, Yes;
SizingPeriod:DesignDay,
  Winter Design Day;
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	diagnostic := findDiagnosticByCodeAndValue(AnalyzeDiagnostics(doc), "missing_required_object", "RunPeriod")
	if diagnostic == nil {
		t.Fatalf("RunPeriod diagnostic not found")
	}
	if diagnostic.Severity != DiagnosticNotice || diagnostic.Source != "simulation_context" {
		t.Fatalf("RunPeriod diagnostic = %#v, want simulation_context notice", diagnostic)
	}
}

func TestDiagnosticsAvoidOutputVariableKeyReferenceFalsePositive(t *testing.T) {
	doc, err := Parse(`
Version, 24.1;
Building, Test;
Timestep, 4;
RunPeriod, Annual, 1, 1, , 12, 31;
SimulationControl, Yes, Yes, No, No, Yes;
Output:Variable,
  Missing Zone Name,
  Zone Mean Air Temperature,
  Hourly;
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	for _, diagnostic := range AnalyzeDiagnostics(doc) {
		if diagnostic.Code == "missing_zone_reference" && diagnostic.Value == "Missing Zone Name" {
			t.Fatalf("Output:Variable key value was treated as a zone reference: %#v", diagnostic)
		}
	}
}

func TestDiagnosticsTagOrphansAsUserQualityNotice(t *testing.T) {
	doc, err := Parse(`
Version, 24.1;
Building, Test;
Timestep, 4;
RunPeriod, Annual, 1, 1, , 12, 31;
SimulationControl, Yes, Yes, No, No, Yes;
Schedule:Constant,
  Unused Schedule,
  ,
  1.0;
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	diagnostic := findDiagnosticByCodeAndValue(AnalyzeDiagnostics(doc), "orphan_object", "Unused Schedule")
	if diagnostic == nil {
		t.Fatalf("orphan diagnostic not found")
	}
	if diagnostic.Severity != DiagnosticNotice || diagnostic.Source != "user_quality_check" {
		t.Fatalf("orphan diagnostic = %#v, want user_quality_check notice", diagnostic)
	}
}

func findDiagnosticByCodeAndValue(diagnostics []Diagnostic, code string, value string) *Diagnostic {
	for index := range diagnostics {
		if diagnostics[index].Code == code && diagnostics[index].Value == value {
			return &diagnostics[index]
		}
		if diagnostics[index].Code == code && diagnostics[index].ObjectName == value {
			return &diagnostics[index]
		}
	}
	return nil
}
