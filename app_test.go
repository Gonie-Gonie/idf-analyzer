package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const appSummaryIDF = `
Version,
  24.1;                    !- Version Identifier

Zone,
  Office;                  !- Name
`

func TestAnalyzeInputTextIncludesSummary(t *testing.T) {
	app := NewApp()
	result, err := app.AnalyzeInputText(appSummaryIDF)
	if err != nil {
		t.Fatalf("AnalyzeInputText() error = %v", err)
	}
	if result.Report == nil {
		t.Fatalf("AnalyzeInputText() report = nil")
	}
	if result.Report.Summary.MetricCount != 50 {
		t.Fatalf("summary metric count = %d, want 50", result.Report.Summary.MetricCount)
	}
	if len(result.Report.Summary.Categories) != 6 {
		t.Fatalf("summary category count = %d, want 6", len(result.Report.Summary.Categories))
	}
	if result.Report.Geometry.ZoneCount != 1 {
		t.Fatalf("geometry zone count = %d, want 1", result.Report.Geometry.ZoneCount)
	}
}

func TestDefaultEnergyPlusSampleAnalyzes(t *testing.T) {
	content, err := os.ReadFile("frontend/dist/samples/RefBldgLargeOfficeNew2004_Chicago.idf")
	if err != nil {
		t.Fatalf("ReadFile(default sample) error = %v", err)
	}
	result, err := NewApp().AnalyzeInputText(string(content))
	if err != nil {
		t.Fatalf("AnalyzeInputText(default sample) error = %v", err)
	}
	if result.Report.ObjectCount < 100 {
		t.Fatalf("default sample object count = %d, want a complex example", result.Report.ObjectCount)
	}
	if result.Report.Geometry.ZoneCount < 10 || result.Report.Geometry.SurfaceCount < 50 || result.Report.Geometry.WindowCount < 10 {
		t.Fatalf("default sample geometry too small: zones=%d surfaces=%d windows=%d", result.Report.Geometry.ZoneCount, result.Report.Geometry.SurfaceCount, result.Report.Geometry.WindowCount)
	}
}

func TestAppAssetHandlerServesSummaryMetricGuides(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/summary-metric-guides", nil)
	response := httptest.NewRecorder()

	appAssetHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("summary metric guide API status = %d, want %d", response.Code, http.StatusOK)
	}
	var guides []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&guides); err != nil {
		t.Fatalf("summary metric guide API did not return JSON: %v", err)
	}
	if len(guides) != 50 {
		t.Fatalf("summary metric guide API returned %d guides, want 50", len(guides))
	}
}
