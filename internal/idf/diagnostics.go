package idf

import (
	"fmt"
	"sort"
	"strings"
)

const (
	DiagnosticError   = "error"
	DiagnosticWarning = "warning"
)

type Diagnostic struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	ObjectIndex int    `json:"objectIndex,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	ObjectName  string `json:"objectName,omitempty"`
	FieldIndex  int    `json:"fieldIndex,omitempty"`
	Field       string `json:"field,omitempty"`
	Value       string `json:"value,omitempty"`
}

type diagnosticRefKind struct {
	code       string
	category   string
	label      string
	ownerTypes func(string) bool
	fieldMatch func(string) bool
}

var diagnosticReferenceKinds = []diagnosticRefKind{
	{
		code:       "missing_schedule_reference",
		category:   "Reference",
		label:      "schedule",
		ownerTypes: isScheduleType,
		fieldMatch: commentHasWords("schedule", "name"),
	},
	{
		code:     "missing_construction_reference",
		category: "Reference",
		label:    "construction",
		ownerTypes: func(objectType string) bool {
			return strings.HasPrefix(strings.ToLower(objectType), "construction")
		},
		fieldMatch: commentHasWords("construction", "name"),
	},
	{
		code:       "missing_material_reference",
		category:   "Reference",
		label:      "material",
		ownerTypes: isMaterialType,
		fieldMatch: commentHasWords("material", "name"),
	},
	{
		code:       "missing_zone_reference",
		category:   "Reference",
		label:      "zone",
		ownerTypes: isZoneReferenceTargetType,
		fieldMatch: commentHasWords("zone", "name"),
	},
}

func AnalyzeDiagnostics(doc Document) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, requiredObjectDiagnostics(doc)...)
	diagnostics = append(diagnostics, duplicateNameDiagnostics(doc)...)
	diagnostics = append(diagnostics, referenceDiagnostics(doc)...)
	diagnostics = append(diagnostics, orphanDiagnostics(doc)...)
	diagnostics = append(diagnostics, geometryDiagnostics(doc)...)
	diagnostics = append(diagnostics, scheduleDiagnostics(doc)...)
	diagnostics = append(diagnostics, hvacNodeDiagnostics(doc)...)

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Severity != diagnostics[j].Severity {
			return diagnostics[i].Severity == DiagnosticError
		}
		if diagnostics[i].Category != diagnostics[j].Category {
			return diagnostics[i].Category < diagnostics[j].Category
		}
		if diagnostics[i].ObjectIndex != diagnostics[j].ObjectIndex {
			return diagnostics[i].ObjectIndex < diagnostics[j].ObjectIndex
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	return diagnostics
}

func requiredObjectDiagnostics(doc Document) []Diagnostic {
	required := []string{"Version", "Building", "Timestep", "RunPeriod", "SimulationControl"}
	present := map[string]bool{}
	for _, obj := range doc.Objects {
		present[strings.ToLower(obj.Type)] = true
	}

	var diagnostics []Diagnostic
	for _, objectType := range required {
		if !present[strings.ToLower(objectType)] {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: DiagnosticError,
				Category: "Required Object",
				Code:     "missing_required_object",
				Message:  fmt.Sprintf("Missing required %s object.", objectType),
				Value:    objectType,
			})
		}
	}
	return diagnostics
}

func duplicateNameDiagnostics(doc Document) []Diagnostic {
	byTypeAndName := map[string][]Object{}
	for _, obj := range doc.Objects {
		name := objectName(obj)
		if name == "" {
			continue
		}
		key := strings.ToLower(obj.Type) + "\x00" + normalizeName(name)
		byTypeAndName[key] = append(byTypeAndName[key], obj)
	}

	var diagnostics []Diagnostic
	for _, objects := range byTypeAndName {
		if len(objects) < 2 {
			continue
		}
		for _, obj := range objects {
			diagnostics = append(diagnostics, diagnosticForObject(DiagnosticError, "Duplicate Name", "duplicate_name", obj,
				fmt.Sprintf("Duplicate %s name %q.", obj.Type, objectName(obj))))
		}
	}
	return diagnostics
}

func referenceDiagnostics(doc Document) []Diagnostic {
	owners := map[string]map[string]bool{}
	for _, kind := range diagnosticReferenceKinds {
		owners[kind.code] = map[string]bool{}
	}

	for _, obj := range doc.Objects {
		name := objectName(obj)
		if name == "" {
			continue
		}
		for _, kind := range diagnosticReferenceKinds {
			if kind.ownerTypes(obj.Type) {
				owners[kind.code][normalizeName(name)] = true
			}
		}
	}

	var diagnostics []Diagnostic
	for _, obj := range doc.Objects {
		for fieldIndex, field := range obj.Fields {
			value := strings.TrimSpace(field.Value)
			if value == "" || strings.EqualFold(value, "autocalculate") {
				continue
			}
			for _, kind := range diagnosticReferenceKinds {
				if !kind.fieldMatch(field.Comment) || kind.ownerTypes(obj.Type) {
					continue
				}
				if !owners[kind.code][normalizeName(value)] {
					diagnostics = append(diagnostics, Diagnostic{
						Severity:    DiagnosticError,
						Category:    kind.category,
						Code:        kind.code,
						Message:     fmt.Sprintf("Missing %s reference %q.", kind.label, value),
						ObjectIndex: obj.Index,
						ObjectType:  obj.Type,
						ObjectName:  objectName(obj),
						FieldIndex:  fieldIndex,
						Field:       field.Comment,
						Value:       value,
					})
				}
			}
		}
	}
	return diagnostics
}

func orphanDiagnostics(doc Document) []Diagnostic {
	unused := FindUnusedObjects(doc)
	diagnostics := make([]Diagnostic, 0, len(unused))
	for _, obj := range unused {
		diagnostics = append(diagnostics, Diagnostic{
			Severity:    DiagnosticWarning,
			Category:    "Orphan Object",
			Code:        "orphan_object",
			Message:     fmt.Sprintf("%s %q is not referenced by other objects.", obj.Type, obj.Name),
			ObjectIndex: obj.Index,
			ObjectType:  obj.Type,
			ObjectName:  obj.Name,
		})
	}
	return diagnostics
}

func geometryDiagnostics(doc Document) []Diagnostic {
	zoneNames := map[string]bool{}
	for _, obj := range doc.Objects {
		if isZoneLikeType(obj.Type) {
			if name := objectName(obj); name != "" {
				zoneNames[normalizeName(name)] = true
			}
		}
	}

	var diagnostics []Diagnostic
	for _, obj := range doc.Objects {
		if !isBuildingSurfaceType(obj.Type) && !isFenestrationType(obj.Type) {
			continue
		}
		vertices, hasVertices := detailedVertices(obj)
		if !hasVertices {
			diagnostics = append(diagnostics, diagnosticForObject(DiagnosticWarning, "Geometry", "invalid_vertices", obj,
				fmt.Sprintf("%s %q does not have enough detailed vertices.", obj.Type, objectLabel(obj))))
			continue
		}
		area, ok := polygonArea(vertices)
		if !ok || area <= 0 {
			diagnostics = append(diagnostics, diagnosticForObject(DiagnosticWarning, "Geometry", "zero_area", obj,
				fmt.Sprintf("%s %q has zero or invalid area.", obj.Type, objectLabel(obj))))
		}
		if isBuildingSurfaceType(obj.Type) {
			zoneName := findFieldByCommentWords(obj, "zone", "name")
			if zoneName == "" || !zoneNames[normalizeName(zoneName)] {
				diagnostics = append(diagnostics, diagnosticForObject(DiagnosticWarning, "Geometry", "surface_unconnected_zone", obj,
					fmt.Sprintf("Surface %q references missing zone %q.", objectLabel(obj), zoneName)))
			}
		}
	}
	return diagnostics
}

func scheduleDiagnostics(doc Document) []Diagnostic {
	referenced := map[string]bool{}
	schedules := map[string]Object{}
	for _, obj := range doc.Objects {
		if isScheduleType(obj.Type) {
			if name := objectName(obj); name != "" {
				schedules[normalizeName(name)] = obj
			}
			continue
		}
		for _, field := range obj.Fields {
			if commentHasWords("schedule", "name")(field.Comment) && strings.TrimSpace(field.Value) != "" {
				referenced[normalizeName(field.Value)] = true
			}
		}
	}

	var diagnostics []Diagnostic
	for key := range referenced {
		schedule, ok := schedules[key]
		if !ok {
			continue
		}
		if _, supported := annualScheduleHours(schedule); !supported {
			diagnostics = append(diagnostics, diagnosticForObject(DiagnosticWarning, "Schedule", "unsupported_annual_hours", schedule,
				fmt.Sprintf("Referenced schedule %q cannot be evaluated for annual operating hours.", objectName(schedule))))
		}
	}
	return diagnostics
}

func hvacNodeDiagnostics(doc Document) []Diagnostic {
	nodeRefs := map[string][]Object{}
	degree := map[string]int{}
	graph := map[string]map[string]bool{}

	for _, obj := range doc.Objects {
		for _, field := range obj.Fields {
			comment := strings.ToLower(field.Comment)
			value := strings.TrimSpace(field.Value)
			if value == "" || !strings.Contains(comment, "node") || !strings.Contains(comment, "name") {
				continue
			}
			nodeRefs[normalizeName(value)] = append(nodeRefs[normalizeName(value)], obj)
		}
		for _, connection := range extractHVACConnections(obj) {
			from := normalizeName(connection.FromNode)
			to := normalizeName(connection.ToNode)
			if from == "" || to == "" {
				continue
			}
			degree[from]++
			degree[to]++
			if graph[from] == nil {
				graph[from] = map[string]bool{}
			}
			if graph[to] == nil {
				graph[to] = map[string]bool{}
			}
			graph[from][to] = true
			graph[to][from] = true
		}
	}

	var diagnostics []Diagnostic
	for node, objects := range nodeRefs {
		if degree[node] == 0 && len(objects) == 1 {
			obj := objects[0]
			diagnostics = append(diagnostics, diagnosticForObject(DiagnosticWarning, "HVAC Node", "unconnected_node", obj,
				fmt.Sprintf("Node %q appears only once and is not connected by inferred inlet/outlet links.", node)))
		}
	}

	if components := graphComponentCount(graph); components > 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: DiagnosticWarning,
			Category: "HVAC Node",
			Code:     "disconnected_node_graph",
			Message:  fmt.Sprintf("Inferred HVAC node graph has %d disconnected components.", components),
		})
	}
	return diagnostics
}

func graphComponentCount(graph map[string]map[string]bool) int {
	visited := map[string]bool{}
	components := 0
	for node := range graph {
		if visited[node] {
			continue
		}
		components++
		stack := []string{node}
		visited[node] = true
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for next := range graph[current] {
				if visited[next] {
					continue
				}
				visited[next] = true
				stack = append(stack, next)
			}
		}
	}
	return components
}

func diagnosticForObject(severity, category, code string, obj Object, message string) Diagnostic {
	return Diagnostic{
		Severity:    severity,
		Category:    category,
		Code:        code,
		Message:     message,
		ObjectIndex: obj.Index,
		ObjectType:  obj.Type,
		ObjectName:  objectName(obj),
	}
}

func commentHasWords(words ...string) func(string) bool {
	return func(comment string) bool {
		comment = strings.ToLower(comment)
		for _, word := range words {
			if !strings.Contains(comment, strings.ToLower(word)) {
				return false
			}
		}
		return true
	}
}

func isZoneLikeType(objectType string) bool {
	return strings.EqualFold(objectType, "Zone") || strings.EqualFold(objectType, "Space")
}

func isZoneReferenceTargetType(objectType string) bool {
	return isZoneLikeType(objectType) || strings.EqualFold(objectType, "ZoneList")
}

func isMaterialType(objectType string) bool {
	lower := strings.ToLower(objectType)
	return strings.HasPrefix(lower, "material") || strings.HasPrefix(lower, "windowmaterial")
}

func objectLabel(obj Object) string {
	if name := objectName(obj); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", obj.Index+1)
}
