package main

import (
	"os"
	"strings"
	"testing"
)

func TestFrontendHVACDefaultUICopyAvoidsDebugAndLegacyTerms(t *testing.T) {
	files := []string{
		"frontend/src/js/hvac-views.js",
		"frontend/src/js/i18n.js",
		"frontend/src/js/state.js",
	}
	forbidden := []string{
		"Rule edges",
		"Rule trace",
		"Rule path",
		"Terminal / Equipment",
		"Plant / Condenser",
		"terminal:direct",
		"terminalComponents",
		"buildRelationGraph",
		"plant-terminal",
		"source-zone",
		`data-hvac-open-view="relation"`,
		"relation-link:",
		"Zone relations",
		"Other loops",
		"hvac.inferred",
		"Inferred",
		"Cross-loop",
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(content)
		for _, term := range forbidden {
			if strings.Contains(text, term) {
				t.Fatalf("%s contains forbidden HVAC default UI copy %q", file, term)
			}
		}
	}
}

func TestFrontendHVACStartsOnZoneServices(t *testing.T) {
	content, err := os.ReadFile("frontend/src/js/state.js")
	if err != nil {
		t.Fatalf("read state.js: %v", err)
	}
	if !strings.Contains(string(content), `activeHVACView: "services"`) {
		t.Fatalf("state.js should default HVAC to Zone Services view")
	}
}

func TestFrontendHVACServiceDOMContracts(t *testing.T) {
	content, err := os.ReadFile("frontend/src/js/hvac-views.js")
	if err != nil {
		t.Fatalf("read hvac views: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		"function buildServiceGraph(paths, couplings)",
		"function bundleServiceGraphLinks",
		"hvac-service-table-row",
		"hvac-service-svg",
		"hvac-edge-bundle-badge",
		"hvac-trace-drawer",
		"evaporative_cooler",
		"renderHVACViewTab(\"services\"",
		"renderHVACViewTab(\"couplings\"",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("hvac service renderer is missing DOM contract %q", required)
		}
	}
}

func TestFrontendHVACServiceStylesCoverRoutingAndBundling(t *testing.T) {
	content, err := os.ReadFile("frontend/src/styles.css")
	if err != nil {
		t.Fatalf("read styles: %v", err)
	}
	text := string(content)
	for _, required := range []string{
		".hvac-graph-link.service.bundled",
		".hvac-edge-bundle-badge",
		".hvac-service-link-group:hover .hvac-edge-label",
		".hvac-graph-link.medium-chilled-water",
		".hvac-graph-link.medium-hot-water",
		".hvac-graph-link.medium-refrigerant",
		".hvac-graph-link.medium-electricity",
		".hvac-graph-link.medium-control",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("hvac service styles are missing %q", required)
		}
	}
}

func TestFrontendHVACRendererAvoidsResolverConfidenceVocabulary(t *testing.T) {
	content, err := os.ReadFile("frontend/src/js/hvac-views.js")
	if err != nil {
		t.Fatalf("read hvac views: %v", err)
	}
	text := strings.ToLower(string(content))
	for _, forbidden := range []string{"confidence", "inferred", "weak", "unsupported"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("hvac renderer contains resolver confidence vocabulary %q", forbidden)
		}
	}
}

func TestFrontendHVACRendererAvoidsLegacyRelationGraphImplementation(t *testing.T) {
	content, err := os.ReadFile("frontend/src/js/hvac-views.js")
	if err != nil {
		t.Fatalf("read hvac views: %v", err)
	}
	text := string(content)
	for _, forbidden := range []string{
		"selected.relations",
		"ruleEdgeCountLabel",
		"ruleEdgeSummary",
		"ruleEdgesForRelation(",
		`t("hvac.terminals"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("hvac renderer still contains legacy relation implementation %q", forbidden)
		}
	}
}
