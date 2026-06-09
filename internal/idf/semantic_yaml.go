package idf

import (
	"fmt"
	"sort"
	"strings"
)

const semanticYAMLSchema = "eplus-semantic/0.1"

type SemanticYAMLMetadata struct {
	EnergyPlusVersion string
	SourceFormat      string
}

type SemanticYAMLProjection struct {
	Schema            string                   `json:"schema"`
	EnergyPlusVersion string                   `json:"energyplusVersion,omitempty"`
	SourceFormat      string                   `json:"sourceFormat,omitempty"`
	Text              string                   `json:"text"`
	Lines             []SemanticYAMLLine       `json:"lines"`
	DuplicateGroups   []SemanticDuplicateGroup `json:"duplicateGroups,omitempty"`
	ObjectCount       int                      `json:"objectCount"`
}

type SemanticYAMLLine struct {
	Text        string `json:"text"`
	Indent      int    `json:"indent"`
	Key         string `json:"key,omitempty"`
	Value       string `json:"value,omitempty"`
	ObjectIndex *int   `json:"objectIndex,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	ObjectName  string `json:"objectName,omitempty"`
	FieldIndex  *int   `json:"fieldIndex,omitempty"`
	Editable    bool   `json:"editable,omitempty"`
	Role        string `json:"role,omitempty"`
}

type SemanticDuplicateGroup struct {
	Group         string `json:"group"`
	ObjectType    string `json:"objectType"`
	Name          string `json:"name"`
	ObjectIndexes []int  `json:"objectIndexes"`
	SyncPolicy    string `json:"syncPolicy"`
	AutoFixable   bool   `json:"autoFixable"`
}

type SemanticDuplicateFix struct {
	ObjectIndex int    `json:"objectIndex"`
	ObjectType  string `json:"objectType"`
	Before      string `json:"before"`
	After       string `json:"after"`
}

type semanticYAMLBuilder struct {
	lines []SemanticYAMLLine
}

func BuildSemanticYAMLProjection(doc Document, metadata SemanticYAMLMetadata) SemanticYAMLProjection {
	duplicates := semanticDuplicateGroups(doc)
	duplicateByObject := semanticDuplicateMap(duplicates)
	objectsBySection := semanticObjectsBySection(doc)
	builder := &semanticYAMLBuilder{}

	builder.raw(0, "semantic_energyplus_model:")
	builder.kv(1, "schema", semanticYAMLSchema)
	if strings.TrimSpace(metadata.EnergyPlusVersion) != "" {
		builder.kv(1, "energyplus_version", strings.TrimSpace(metadata.EnergyPlusVersion))
	} else {
		builder.kv(1, "energyplus_version", "unknown")
	}
	builder.kv(1, "yaml_profile", "strict-yaml-1.2-json-compatible")

	builder.raw(1, "project:")
	builder.kv(2, "source_format", blankAs(metadata.SourceFormat, "unknown"))
	builder.kv(2, "object_count", fmt.Sprintf("%d", len(doc.Objects)))
	builder.kv(2, "semantic_policy", "projection_over_idf_object_registry")

	writeSemanticSections(builder, objectsBySection, duplicateByObject)
	writeSemanticDuplicateGroups(builder, duplicates)
	writeSemanticSourcePreservation(builder, doc)

	textLines := make([]string, len(builder.lines))
	for index, line := range builder.lines {
		textLines[index] = line.Text
	}
	return SemanticYAMLProjection{
		Schema:            semanticYAMLSchema,
		EnergyPlusVersion: strings.TrimSpace(metadata.EnergyPlusVersion),
		SourceFormat:      strings.TrimSpace(metadata.SourceFormat),
		Text:              strings.Join(textLines, "\n") + "\n",
		Lines:             builder.lines,
		DuplicateGroups:   duplicates,
		ObjectCount:       len(doc.Objects),
	}
}

func ApplySemanticDuplicateNameFixes(doc Document) (Document, []SemanticDuplicateFix) {
	updated := doc.clone()
	seenByType := map[string]map[string]bool{}
	reservedByType := semanticReservedNamesByType(updated)
	nextCountByTypeName := map[string]int{}
	var fixes []SemanticDuplicateFix

	for index := range updated.Objects {
		obj := &updated.Objects[index]
		name := objectName(*obj)
		if strings.TrimSpace(name) == "" || len(obj.Fields) == 0 {
			continue
		}
		typeKey := normalizeName(obj.Type)
		if seenByType[typeKey] == nil {
			seenByType[typeKey] = map[string]bool{}
		}
		nameKey := normalizeName(name)
		groupKey := typeKey + "/" + nameKey
		if !seenByType[typeKey][nameKey] {
			seenByType[typeKey][nameKey] = true
			nextCountByTypeName[groupKey] = 2
			continue
		}
		nextName := semanticUniqueName(name, reservedByType[typeKey], nextCountByTypeName[groupKey])
		nextCountByTypeName[groupKey]++
		obj.Fields[0].Value = nextName
		seenByType[typeKey][normalizeName(nextName)] = true
		reservedByType[typeKey][normalizeName(nextName)] = true
		fixes = append(fixes, SemanticDuplicateFix{
			ObjectIndex: obj.Index,
			ObjectType:  obj.Type,
			Before:      name,
			After:       nextName,
		})
	}
	reindexObjects(&updated)
	return updated, fixes
}

func semanticReservedNamesByType(doc Document) map[string]map[string]bool {
	reserved := map[string]map[string]bool{}
	for _, obj := range doc.Objects {
		name := objectName(obj)
		if strings.TrimSpace(name) == "" {
			continue
		}
		typeKey := normalizeName(obj.Type)
		if reserved[typeKey] == nil {
			reserved[typeKey] = map[string]bool{}
		}
		reserved[typeKey][normalizeName(name)] = true
	}
	return reserved
}

func semanticUniqueName(base string, existing map[string]bool, start int) string {
	for index := start; ; index++ {
		candidate := fmt.Sprintf("%s %d", strings.TrimSpace(base), index)
		if !existing[normalizeName(candidate)] {
			return candidate
		}
	}
}

func writeSemanticSections(builder *semanticYAMLBuilder, sections map[string][]Object, duplicateByObject map[int]SemanticDuplicateGroup) {
	ordered := []struct {
		key   string
		label string
	}{
		{key: "simulation", label: "simulation"},
		{key: "site", label: "site"},
		{key: "building", label: "building"},
		{key: "schedules", label: "schedules"},
		{key: "constructions", label: "constructions"},
		{key: "zones", label: "zones"},
		{key: "hvac", label: "hvac"},
		{key: "outputs", label: "outputs"},
		{key: "miscellaneous", label: "miscellaneous"},
	}
	for _, section := range ordered {
		objects := sections[section.key]
		if len(objects) == 0 {
			builder.raw(1, section.label+": {}")
			continue
		}
		builder.raw(1, section.label+":")
		builder.raw(2, "objects:")
		for _, obj := range objects {
			writeSemanticObject(builder, obj, duplicateByObject[obj.Index])
		}
	}
}

func writeSemanticObject(builder *semanticYAMLBuilder, obj Object, duplicate SemanticDuplicateGroup) {
	objectIndex := obj.Index
	name := objectName(obj)
	builder.objectKV(3, "- class", obj.Type, objectIndex, obj.Type, name)
	if name != "" && len(obj.Fields) > 0 {
		fieldIndex := 0
		builder.fieldKV(4, "name", name, objectIndex, obj.Type, name, fieldIndex)
	}
	builder.kvForObject(4, "source_order", fmt.Sprintf("%d", obj.Index+1), objectIndex, obj.Type, name)
	builder.rawForObject(4, "source_fields:", objectIndex, obj.Type, name)
	for fieldIndex, field := range obj.Fields {
		if fieldIndex == 0 && name != "" {
			continue
		}
		key := semanticFieldKey(field, fieldIndex)
		builder.fieldKV(5, key, field.Value, objectIndex, obj.Type, name, fieldIndex)
	}
	if duplicate.Group != "" {
		builder.rawForObject(4, "duplicated_as:", objectIndex, obj.Type, name)
		builder.kvForObject(5, "group", duplicate.Group, objectIndex, obj.Type, name)
		builder.kvForObject(5, "role_here", semanticDuplicateRole(duplicate, objectIndex), objectIndex, obj.Type, name)
		builder.kvForObject(5, "sync_policy", duplicate.SyncPolicy, objectIndex, obj.Type, name)
	}
}

func writeSemanticDuplicateGroups(builder *semanticYAMLBuilder, duplicates []SemanticDuplicateGroup) {
	if len(duplicates) == 0 {
		builder.raw(1, "duplicate_groups: []")
		return
	}
	builder.raw(1, "duplicate_groups:")
	for _, group := range duplicates {
		builder.raw(2, "- group: "+yamlScalar(group.Group))
		builder.kv(3, "object_type", group.ObjectType)
		builder.kv(3, "name", group.Name)
		builder.raw(3, "object_indexes:")
		for _, index := range group.ObjectIndexes {
			builder.raw(4, "- "+fmt.Sprintf("%d", index))
		}
		builder.kv(3, "sync_policy", group.SyncPolicy)
		builder.kv(3, "auto_fixable", fmt.Sprintf("%t", group.AutoFixable))
	}
}

func writeSemanticSourcePreservation(builder *semanticYAMLBuilder, doc Document) {
	builder.raw(1, "raw_extensions:")
	builder.kv(2, "export_policy", "preserve_current_registry")
	builder.kv(2, "object_count", fmt.Sprintf("%d", len(doc.Objects)))
	builder.raw(2, "objects:")
	for _, obj := range doc.Objects {
		name := objectName(obj)
		builder.objectKV(3, "- class", obj.Type, obj.Index, obj.Type, name)
		if name != "" {
			builder.kvForObject(4, "name", name, obj.Index, obj.Type, name)
		}
		builder.kvForObject(4, "source_order", fmt.Sprintf("%d", obj.Index+1), obj.Index, obj.Type, name)
		builder.kvForObject(4, "field_count", fmt.Sprintf("%d", len(obj.Fields)), obj.Index, obj.Type, name)
	}
	builder.raw(1, "source_preservation:")
	builder.kv(2, "object_order", "preserved")
	builder.kv(2, "field_order", "preserved")
	builder.kv(2, "comments", "best_effort_from_current_parser")
	builder.kv(2, "roundtrip_scope", "idf_object_registry")
}

func semanticObjectsBySection(doc Document) map[string][]Object {
	out := map[string][]Object{}
	for _, obj := range doc.Objects {
		section := semanticSectionForType(obj.Type)
		out[section] = append(out[section], obj)
	}
	return out
}

func semanticSectionForType(objectType string) string {
	lower := strings.ToLower(strings.TrimSpace(objectType))
	switch {
	case lower == "version" ||
		lower == "simulationcontrol" ||
		lower == "timestep" ||
		strings.Contains(lower, "algorithm"):
		return "simulation"
	case strings.HasPrefix(lower, "site:") ||
		strings.HasPrefix(lower, "sizingperiod:") ||
		lower == "runperiod":
		return "site"
	case lower == "building" ||
		lower == "globalgeometryrules" ||
		strings.HasPrefix(lower, "shading:"):
		return "building"
	case strings.HasPrefix(lower, "schedule:"):
		return "schedules"
	case lower == "construction" ||
		strings.HasPrefix(lower, "construction:") ||
		strings.HasPrefix(lower, "material") ||
		strings.HasPrefix(lower, "windowmaterial"):
		return "constructions"
	case lower == "zone" ||
		lower == "space" ||
		strings.Contains(lower, "surface") ||
		lower == "people" ||
		lower == "lights" ||
		strings.Contains(lower, "equipment") ||
		strings.HasPrefix(lower, "zoneinfiltration:") ||
		strings.HasPrefix(lower, "zoneventilation:") ||
		strings.HasPrefix(lower, "zonecontrol:") ||
		strings.HasPrefix(lower, "thermostatsetpoint:"):
		return "zones"
	case strings.Contains(lower, "hvac") ||
		strings.HasPrefix(lower, "airloop") ||
		strings.HasPrefix(lower, "plantloop") ||
		strings.HasPrefix(lower, "branch") ||
		strings.HasPrefix(lower, "connector") ||
		strings.HasPrefix(lower, "node") ||
		strings.HasPrefix(lower, "coil:") ||
		strings.HasPrefix(lower, "fan:") ||
		strings.HasPrefix(lower, "pump:") ||
		strings.HasPrefix(lower, "boiler:") ||
		strings.HasPrefix(lower, "chiller:") ||
		strings.HasPrefix(lower, "controller:") ||
		strings.HasPrefix(lower, "setpointmanager:") ||
		strings.HasPrefix(lower, "pipe:"):
		return "hvac"
	case strings.HasPrefix(lower, "output:") ||
		strings.HasPrefix(lower, "outputcontrol:") ||
		strings.HasPrefix(lower, "meter:"):
		return "outputs"
	default:
		return "miscellaneous"
	}
}

func semanticDuplicateGroups(doc Document) []SemanticDuplicateGroup {
	type item struct {
		objectType string
		name       string
		indexes    []int
	}
	byKey := map[string]*item{}
	for _, obj := range doc.Objects {
		name := objectName(obj)
		if name == "" {
			continue
		}
		key := normalizeName(obj.Type) + "/" + normalizeName(name)
		if byKey[key] == nil {
			byKey[key] = &item{objectType: obj.Type, name: name}
		}
		byKey[key].indexes = append(byKey[key].indexes, obj.Index)
	}
	var groups []SemanticDuplicateGroup
	for _, item := range byKey {
		if len(item.indexes) < 2 {
			continue
		}
		sort.Ints(item.indexes)
		groups = append(groups, SemanticDuplicateGroup{
			Group:         semanticDuplicateGroupID(item.objectType, item.name),
			ObjectType:    item.objectType,
			Name:          item.name,
			ObjectIndexes: append([]int(nil), item.indexes...),
			SyncPolicy:    "edit_once_sync_all",
			AutoFixable:   true,
		})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].ObjectType != groups[j].ObjectType {
			return strings.ToLower(groups[i].ObjectType) < strings.ToLower(groups[j].ObjectType)
		}
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	return groups
}

func semanticDuplicateMap(groups []SemanticDuplicateGroup) map[int]SemanticDuplicateGroup {
	out := map[int]SemanticDuplicateGroup{}
	for _, group := range groups {
		for _, index := range group.ObjectIndexes {
			out[index] = group
		}
	}
	return out
}

func semanticDuplicateGroupID(objectType string, name string) string {
	parts := strings.Fields(strings.ToLower(objectType + " " + name))
	raw := strings.Join(parts, "-")
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	value := strings.Trim(b.String(), "-")
	if value == "" {
		return "duplicate-object"
	}
	return "duplicate-" + value
}

func semanticDuplicateRole(group SemanticDuplicateGroup, objectIndex int) string {
	if len(group.ObjectIndexes) > 0 && group.ObjectIndexes[0] == objectIndex {
		return "primary_projection"
	}
	return "duplicate_projection"
}

func semanticFieldKey(field Field, fieldIndex int) string {
	key := strings.TrimSpace(field.Comment)
	if key == "" {
		key = fmt.Sprintf("field_%d", fieldIndex+1)
	}
	key = strings.TrimSpace(strings.Split(key, "{")[0])
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(key) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return fmt.Sprintf("field_%d", fieldIndex+1)
	}
	return out
}

func (builder *semanticYAMLBuilder) raw(indent int, raw string) {
	builder.lines = append(builder.lines, SemanticYAMLLine{Text: semanticLineText(indent, raw), Indent: indent, Role: "syntax"})
}

func (builder *semanticYAMLBuilder) kv(indent int, key string, value string) {
	builder.lines = append(builder.lines, SemanticYAMLLine{
		Text:   semanticLineText(indent, key+": "+yamlScalar(value)),
		Indent: indent,
		Key:    key,
		Value:  value,
		Role:   "metadata",
	})
}

func (builder *semanticYAMLBuilder) rawForObject(indent int, raw string, objectIndex int, objectType string, objectName string) {
	builder.lines = append(builder.lines, SemanticYAMLLine{
		Text:        semanticLineText(indent, raw),
		Indent:      indent,
		ObjectIndex: intPtr(objectIndex),
		ObjectType:  objectType,
		ObjectName:  objectName,
		Role:        "object",
	})
}

func (builder *semanticYAMLBuilder) kvForObject(indent int, key string, value string, objectIndex int, objectType string, objectName string) {
	builder.lines = append(builder.lines, SemanticYAMLLine{
		Text:        semanticLineText(indent, key+": "+yamlScalar(value)),
		Indent:      indent,
		Key:         key,
		Value:       value,
		ObjectIndex: intPtr(objectIndex),
		ObjectType:  objectType,
		ObjectName:  objectName,
		Role:        "object",
	})
}

func (builder *semanticYAMLBuilder) objectKV(indent int, key string, value string, objectIndex int, objectType string, objectName string) {
	builder.lines = append(builder.lines, SemanticYAMLLine{
		Text:        semanticLineText(indent, key+": "+yamlScalar(value)),
		Indent:      indent,
		Key:         strings.TrimPrefix(key, "- "),
		Value:       value,
		ObjectIndex: intPtr(objectIndex),
		ObjectType:  objectType,
		ObjectName:  objectName,
		Role:        "object",
	})
}

func (builder *semanticYAMLBuilder) fieldKV(indent int, key string, value string, objectIndex int, objectType string, objectName string, fieldIndex int) {
	builder.lines = append(builder.lines, SemanticYAMLLine{
		Text:        semanticLineText(indent, key+": "+yamlScalar(value)),
		Indent:      indent,
		Key:         key,
		Value:       value,
		ObjectIndex: intPtr(objectIndex),
		ObjectType:  objectType,
		ObjectName:  objectName,
		FieldIndex:  intPtr(fieldIndex),
		Editable:    true,
		Role:        "field",
	})
}

func semanticLineText(indent int, raw string) string {
	return strings.Repeat("  ", indent) + raw
}

func yamlScalar(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "null"
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" || lower == "yes" || lower == "no" || lower == "on" || lower == "off" || lower == "null" {
		return quoteYAMLString(value)
	}
	if strings.ContainsAny(value, ",:[]{}#*!|>&%@`\"'") || strings.Contains(value, "  ") {
		return quoteYAMLString(value)
	}
	return value
}

func quoteYAMLString(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

func blankAs(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func intPtr(value int) *int {
	return &value
}
