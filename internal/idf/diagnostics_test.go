package idf

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestDiagnosticsGoldenSnapshots(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "valid_baseline",
			input: `
Version, 24.1;
Building, Test;
Timestep, 4;
RunPeriod, Annual, 1, 1, , 12, 31;
SimulationControl, Yes, Yes, No, No, Yes;
`,
			want: "total=0",
		},
		{
			name: "design_day_only_runperiod_notice",
			input: `
Version, 24.1;
Building, Test;
Timestep, 4;
SimulationControl, Yes, Yes, No, No, Yes;
SizingPeriod:DesignDay,
  Winter Design Day;
`,
			want: "total=1\nnotice|simulation_context|missing_required_object|RunPeriod=1",
		},
		{
			name: "output_wildcard_environment_keys",
			input: `
Version, 24.1;
Building, Test;
Timestep, 4;
RunPeriod, Annual, 1, 1, , 12, 31;
SimulationControl, Yes, Yes, No, No, Yes;
Output:Variable, *, Zone Mean Air Temperature, Hourly;
Output:Variable, Environment, Site Outdoor Air Drybulb Temperature, Hourly;
`,
			want: "total=0",
		},
		{
			name: "schedule_compact_tokens",
			input: `
Version, 24.1;
Building, Test;
Timestep, 4;
RunPeriod, Annual, 1, 1, , 12, 31;
SimulationControl, Yes, Yes, No, No, Yes;
Schedule:Compact,
  Always On Compact,
  ,
  Through: 12/31,
  For: AllDays,
  Until: 24:00,
  1.0;
`,
			want: "total=1\nnotice|user_quality_check|orphan_object|Always On Compact=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			got := diagnosticSnapshot(AnalyzeDiagnostics(doc))
			if got != tt.want {
				t.Fatalf("diagnostic snapshot mismatch\nwant:\n%s\n\ngot:\n%s", tt.want, got)
			}
		})
	}
}

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

func diagnosticSnapshot(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return "total=0"
	}
	counts := map[string]int{}
	for _, diagnostic := range diagnostics {
		key := strings.Join([]string{
			diagnostic.Severity,
			diagnostic.Source,
			diagnostic.Code,
			firstNonEmpty(diagnostic.Value, diagnostic.ObjectName, diagnostic.ObjectType),
		}, "|")
		counts[key]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := []string{fmt.Sprintf("total=%d", len(diagnostics))}
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(lines, "\n")
}
