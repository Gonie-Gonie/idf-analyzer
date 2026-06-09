package idf

import (
	"fmt"
	"sort"
	"strings"
)

const semanticYAMLSchema = "eplus-semantic/0.2"

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
	ctx := buildSemanticContext(doc)
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
	builder.kv(2, "semantic_policy", "semantic_view_over_idf_object_registry")

	writeSemanticObjectLibrary(builder, ctx, "simulation", []string{"Version", "SimulationControl", "Timestep"})
	writeSemanticObjectLibrary(builder, ctx, "site", []string{"Site:Location", "SizingPeriod:DesignDay", "SizingPeriod:WeatherFileDays", "SizingPeriod:WeatherFileConditionType", "RunPeriod"})
	writeSemanticObjectLibrary(builder, ctx, "building", []string{"Building", "GlobalGeometryRules"})
	writeSemanticObjectLibrary(builder, ctx, "schedules", semanticObjectTypesWithPrefix(doc, "Schedule:"))
	writeSemanticConstructions(builder, ctx)
	writeSemanticZones(builder, ctx)
	writeSemanticHVAC(builder, ctx)
	writeSemanticOutputs(builder, ctx)
	writeSemanticSourceNameConflicts(builder, duplicates)
	writeSemanticMiscellaneous(builder, ctx)
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

type semanticContext struct {
	doc              Document
	objectByIndex    map[int]Object
	mapped           map[int]bool
	geometry         GeometryReport
	hvac             HVACReport
	output           OutputReport
	surfacesByZone   map[string][]GeometrySurface
	windowsBySurface map[string][]GeometryWindow
	loadsByZone      map[string]map[string][]Object
	controlsByZone   map[string][]Object
	outputsByTarget  map[string][]OutputObjectSummary
}

func buildSemanticContext(doc Document) *semanticContext {
	ctx := &semanticContext{
		doc:              doc,
		objectByIndex:    map[int]Object{},
		mapped:           map[int]bool{},
		geometry:         AnalyzeGeometry(doc),
		hvac:             AnalyzeHVAC(doc),
		output:           AnalyzeOutput(doc),
		surfacesByZone:   map[string][]GeometrySurface{},
		windowsBySurface: map[string][]GeometryWindow{},
		loadsByZone:      map[string]map[string][]Object{},
		controlsByZone:   map[string][]Object{},
		outputsByTarget:  map[string][]OutputObjectSummary{},
	}
	for _, obj := range doc.Objects {
		ctx.objectByIndex[obj.Index] = obj
	}
	for _, surface := range ctx.geometry.Surfaces {
		ctx.surfacesByZone[normalizeName(surface.ZoneName)] = append(ctx.surfacesByZone[normalizeName(surface.ZoneName)], surface)
	}
	for _, window := range ctx.geometry.Windows {
		ctx.windowsBySurface[normalizeName(window.BaseSurfaceName)] = append(ctx.windowsBySurface[normalizeName(window.BaseSurfaceName)], window)
	}
	for _, summary := range ctx.output.Existing {
		key := normalizeName(summary.KeyValue)
		if key != "" && key != "*" {
			ctx.outputsByTarget[key] = append(ctx.outputsByTarget[key], summary)
		}
	}
	for _, obj := range doc.Objects {
		zoneName, bucket, ok := semanticZoneAttachment(obj)
		if ok {
			key := normalizeName(zoneName)
			if ctx.loadsByZone[key] == nil {
				ctx.loadsByZone[key] = map[string][]Object{}
			}
			ctx.loadsByZone[key][bucket] = append(ctx.loadsByZone[key][bucket], obj)
			continue
		}
		if zoneName := semanticControlZone(obj); zoneName != "" {
			ctx.controlsByZone[normalizeName(zoneName)] = append(ctx.controlsByZone[normalizeName(zoneName)], obj)
		}
	}
	return ctx
}

func (ctx *semanticContext) mark(objectIndex int) {
	if objectIndex >= 0 {
		ctx.mapped[objectIndex] = true
	}
}

func writeSemanticObjectLibrary(builder *semanticYAMLBuilder, ctx *semanticContext, section string, objectTypes []string) {
	objects := semanticObjectsForTypes(ctx.doc, objectTypes)
	if len(objects) == 0 {
		builder.raw(1, section+": {}")
		return
	}
	builder.raw(1, section+":")
	for _, obj := range objects {
		ctx.mark(obj.Index)
		writeSemanticReferenceObject(builder, 2, obj)
	}
}

func writeSemanticConstructions(builder *semanticYAMLBuilder, ctx *semanticContext) {
	if len(ctx.geometry.Constructions) == 0 {
		builder.raw(1, "constructions: {}")
		return
	}
	builder.raw(1, "constructions:")
	for _, construction := range ctx.geometry.Constructions {
		ctx.mark(construction.ObjectIndex)
		name := construction.Name
		builder.fieldKV(2, "- name", name, construction.ObjectIndex, construction.ObjectType, name, 0)
		builder.kvForObject(3, "class", construction.ObjectType, construction.ObjectIndex, construction.ObjectType, name)
		builder.kvForObject(3, "source_object_index", fmt.Sprintf("%d", construction.ObjectIndex), construction.ObjectIndex, construction.ObjectType, name)
		if len(construction.Layers) > 0 {
			builder.rawForObject(3, "layers:", construction.ObjectIndex, construction.ObjectType, name)
			for _, layer := range construction.Layers {
				if layer.ObjectIndex >= 0 {
					ctx.mark(layer.ObjectIndex)
				}
				layerName := blankAs(layer.Name, "unnamed_layer")
				builder.rawForObject(4, "- name: "+yamlScalar(layerName), construction.ObjectIndex, construction.ObjectType, name)
				if layer.ObjectType != "" {
					builder.kvForObject(5, "class", layer.ObjectType, construction.ObjectIndex, construction.ObjectType, name)
				}
				if layer.HasThickness {
					builder.kvForObject(5, "thickness", semanticQuantity(layer.Thickness, "m"), construction.ObjectIndex, construction.ObjectType, name)
				}
			}
		}
	}
}

func writeSemanticZones(builder *semanticYAMLBuilder, ctx *semanticContext) {
	zones := semanticZones(ctx)
	if len(zones) == 0 {
		builder.raw(1, "zones: []")
		return
	}
	builder.raw(1, "zones:")
	for _, zone := range zones {
		ctx.mark(zone.ObjectIndex)
		zoneObj := ctx.objectByIndex[zone.ObjectIndex]
		zoneName := zone.Name
		if zoneName == "" {
			zoneName = objectName(zoneObj)
		}
		builder.fieldKV(2, "- name", zoneName, zone.ObjectIndex, "Zone", zoneName, 0)
		builder.kvForObject(3, "class", "Zone", zone.ObjectIndex, "Zone", zoneName)
		builder.rawForObject(3, "source:", zone.ObjectIndex, "Zone", zoneName)
		builder.kvForObject(4, "object_index", fmt.Sprintf("%d", zone.ObjectIndex), zone.ObjectIndex, "Zone", zoneName)
		builder.kvForObject(4, "object_type", "Zone", zone.ObjectIndex, "Zone", zoneName)
		writeSemanticZoneGeometry(builder, ctx, zoneName)
		writeSemanticZoneLoads(builder, ctx, zoneName)
		writeSemanticZoneControls(builder, ctx, zoneName)
		writeSemanticZoneHVAC(builder, ctx, zoneName)
		writeSemanticAttachedOutputs(builder, 3, "outputs", ctx.outputsByTarget[normalizeName(zoneName)])
	}
}

func writeSemanticZoneGeometry(builder *semanticYAMLBuilder, ctx *semanticContext, zoneName string) {
	surfaces := ctx.surfacesByZone[normalizeName(zoneName)]
	if len(surfaces) == 0 {
		return
	}
	builder.raw(3, "geometry:")
	builder.raw(4, "surfaces:")
	for _, surface := range surfaces {
		ctx.mark(surface.ObjectIndex)
		builder.fieldKV(5, "- name", surface.Name, surface.ObjectIndex, surface.Type, surface.Name, 0)
		builder.kvForObject(6, "class", surface.Type, surface.ObjectIndex, surface.Type, surface.Name)
		builder.kvForObject(6, "type", surface.SurfaceType, surface.ObjectIndex, surface.Type, surface.Name)
		semanticFieldByNames(builder, 6, "construction", ctx.objectByIndex[surface.ObjectIndex], surface.Construction, "Construction Name")
		builder.rawForObject(6, "boundary:", surface.ObjectIndex, surface.Type, surface.Name)
		semanticFieldByNames(builder, 7, "condition", ctx.objectByIndex[surface.ObjectIndex], surface.OutsideBoundary, "Outside Boundary Condition")
		builder.rawForObject(6, "vertices: "+semanticVertices(surface.Vertices), surface.ObjectIndex, surface.Type, surface.Name)
		windows := ctx.windowsBySurface[normalizeName(surface.Name)]
		if len(windows) > 0 {
			builder.rawForObject(6, "fenestration:", surface.ObjectIndex, surface.Type, surface.Name)
			for _, window := range windows {
				ctx.mark(window.ObjectIndex)
				builder.fieldKV(7, "- name", window.Name, window.ObjectIndex, window.Type, window.Name, 0)
				builder.kvForObject(8, "class", window.Type, window.ObjectIndex, window.Type, window.Name)
				builder.kvForObject(8, "type", window.SurfaceType, window.ObjectIndex, window.Type, window.Name)
				semanticFieldByNames(builder, 8, "construction", ctx.objectByIndex[window.ObjectIndex], window.Construction, "Construction Name")
				semanticFieldByNames(builder, 8, "base_surface", ctx.objectByIndex[window.ObjectIndex], window.BaseSurfaceName, "Building Surface Name", "Surface Name")
				builder.rawForObject(8, "vertices: "+semanticVertices(window.Vertices), window.ObjectIndex, window.Type, window.Name)
				writeSemanticAttachedOutputs(builder, 8, "outputs", ctx.outputsByTarget[normalizeName(window.Name)])
			}
		}
		writeSemanticAttachedOutputs(builder, 6, "outputs", ctx.outputsByTarget[normalizeName(surface.Name)])
	}
}

func writeSemanticZoneLoads(builder *semanticYAMLBuilder, ctx *semanticContext, zoneName string) {
	buckets := ctx.loadsByZone[normalizeName(zoneName)]
	if len(buckets) == 0 {
		return
	}
	builder.raw(3, "loads:")
	order := []string{"people", "lights", "electric_equipment", "gas_equipment", "infiltration", "ventilation", "mixing"}
	for _, bucket := range order {
		objects := buckets[bucket]
		if len(objects) == 0 {
			continue
		}
		builder.raw(4, bucket+":")
		for _, obj := range objects {
			ctx.mark(obj.Index)
			writeSemanticLoadObject(builder, 5, obj)
		}
	}
}

func writeSemanticLoadObject(builder *semanticYAMLBuilder, indent int, obj Object) {
	name := objectName(obj)
	builder.fieldKV(indent, "- name", name, obj.Index, obj.Type, name, 0)
	builder.kvForObject(indent+1, "class", obj.Type, obj.Index, obj.Type, name)
	if value, fieldIndex, ok := semanticFieldValue(obj, "Zone or ZoneList Name", "Zone Name"); ok {
		builder.fieldKV(indent+1, "zone", value, obj.Index, obj.Type, name, fieldIndex)
	}
	if value, fieldIndex, ok := semanticFieldValue(obj, "Number of People Schedule Name", "Schedule Name"); ok {
		builder.fieldKV(indent+1, "schedule", value, obj.Index, obj.Type, name, fieldIndex)
	}
	if value, fieldIndex, ok := semanticFieldValue(obj, "Activity Level Schedule Name"); ok {
		builder.fieldKV(indent+1, "activity_schedule", value, obj.Index, obj.Type, name, fieldIndex)
	}
	if value, fieldIndex, ok := semanticLoadLevel(obj); ok {
		builder.fieldKV(indent+1, "level", value, obj.Index, obj.Type, name, fieldIndex)
	}
	writeSemanticAttachedOutputs(builder, indent+1, "outputs", nil)
}

func writeSemanticZoneControls(builder *semanticYAMLBuilder, ctx *semanticContext, zoneName string) {
	controls := ctx.controlsByZone[normalizeName(zoneName)]
	if len(controls) == 0 {
		return
	}
	builder.raw(3, "controls:")
	for _, obj := range controls {
		ctx.mark(obj.Index)
		name := objectName(obj)
		builder.fieldKV(4, "- name", name, obj.Index, obj.Type, name, 0)
		builder.kvForObject(5, "class", obj.Type, obj.Index, obj.Type, name)
		if value, fieldIndex, ok := semanticFieldValue(obj, "Zone Name"); ok {
			builder.fieldKV(5, "zone", value, obj.Index, obj.Type, name, fieldIndex)
		}
	}
}

func writeSemanticZoneHVAC(builder *semanticYAMLBuilder, ctx *semanticContext, zoneName string) {
	var relation HVACZoneChain
	found := false
	for _, candidate := range ctx.hvac.ZoneRelations {
		if strings.EqualFold(candidate.ZoneName, zoneName) {
			relation = candidate
			found = true
			break
		}
	}
	if !found || (len(relation.TerminalUnits) == 0 && len(relation.ZoneEquipment) == 0 && len(relation.AirLoopNames) == 0 && len(relation.PlantLoopNames) == 0) {
		return
	}
	builder.raw(3, "hvac:")
	if len(relation.AirLoopNames) > 0 {
		builder.raw(4, "air_loops:")
		for _, name := range relation.AirLoopNames {
			builder.raw(5, "- "+yamlScalar(name))
		}
	}
	if len(relation.TerminalUnits) > 0 {
		builder.raw(4, "terminals:")
		for _, component := range relation.TerminalUnits {
			ctx.mark(component.ObjectIndex)
			writeSemanticHVACComponent(builder, 5, component, "zone_terminal")
		}
	}
	if len(relation.ZoneEquipment) > 0 {
		builder.raw(4, "equipment:")
		for _, component := range relation.ZoneEquipment {
			ctx.mark(component.ObjectIndex)
			writeSemanticHVACComponent(builder, 5, component, "zone_equipment")
		}
	}
}

func writeSemanticHVAC(builder *semanticYAMLBuilder, ctx *semanticContext) {
	if len(ctx.hvac.Loops) == 0 {
		builder.raw(1, "hvac: {}")
		return
	}
	builder.raw(1, "hvac:")
	writeSemanticHVACLoops(builder, ctx, "air_loops", "air")
	writeSemanticHVACLoops(builder, ctx, "plant_loops", "plant")
}

func writeSemanticHVACLoops(builder *semanticYAMLBuilder, ctx *semanticContext, key string, loopType string) {
	var loops []HVACLoop
	for _, loop := range ctx.hvac.Loops {
		if strings.EqualFold(loop.Type, loopType) {
			loops = append(loops, loop)
		}
	}
	if len(loops) == 0 {
		return
	}
	builder.raw(2, key+":")
	for _, loop := range loops {
		ctx.mark(loop.ObjectIndex)
		builder.fieldKV(3, "- name", loop.Name, loop.ObjectIndex, loop.Type, loop.Name, 0)
		builder.kvForObject(4, "class", loop.Type, loop.ObjectIndex, loop.Type, loop.Name)
		writeSemanticHVACSide(builder, ctx, 4, "supply_side", loop.SupplySide)
		writeSemanticHVACSide(builder, ctx, 4, "demand_side", loop.DemandSide)
	}
}

func writeSemanticHVACSide(builder *semanticYAMLBuilder, ctx *semanticContext, indent int, key string, side HVACLoopSide) {
	if side.Name == "" && len(side.Branches) == 0 {
		return
	}
	builder.raw(indent, key+":")
	if side.InletNode != "" {
		builder.kv(indent+1, "inlet_node", side.InletNode)
	}
	if side.OutletNode != "" {
		builder.kv(indent+1, "outlet_node", side.OutletNode)
	}
	if len(side.Branches) > 0 {
		builder.raw(indent+1, "branches:")
		for _, branch := range side.Branches {
			ctx.mark(branch.ObjectIndex)
			builder.rawForObject(indent+2, "- name: "+yamlScalar(branch.Name), branch.ObjectIndex, "Branch", branch.Name)
			if len(branch.Components) > 0 {
				builder.rawForObject(indent+3, "components:", branch.ObjectIndex, "Branch", branch.Name)
				for _, component := range branch.Components {
					ctx.mark(component.ObjectIndex)
					writeSemanticHVACComponent(builder, indent+4, component, "loop_component")
				}
			}
		}
	}
}

func writeSemanticHVACComponent(builder *semanticYAMLBuilder, indent int, component HVACComponent, role string) {
	name := component.ObjectName
	objectIndex := component.ObjectIndex
	builder.rawForObject(indent, "- name: "+yamlScalar(name), objectIndex, component.ObjectType, name)
	builder.kvForObject(indent+1, "class", component.ObjectType, objectIndex, component.ObjectType, name)
	if component.InletNode != "" {
		builder.kvForObject(indent+1, "inlet_node", component.InletNode, objectIndex, component.ObjectType, name)
	}
	if component.OutletNode != "" {
		builder.kvForObject(indent+1, "outlet_node", component.OutletNode, objectIndex, component.ObjectType, name)
	}
	if objectIndex >= 0 {
		builder.rawForObject(indent+1, "duplicated_as:", objectIndex, component.ObjectType, name)
		builder.kvForObject(indent+2, "group", "obj-"+fmt.Sprintf("%d", objectIndex), objectIndex, component.ObjectType, name)
		builder.kvForObject(indent+2, "role_here", role, objectIndex, component.ObjectType, name)
		builder.kvForObject(indent+2, "sync_policy", "edit_once_sync_all", objectIndex, component.ObjectType, name)
	}
}

func writeSemanticOutputs(builder *semanticYAMLBuilder, ctx *semanticContext) {
	builder.raw(1, "outputs:")
	builder.raw(2, "files:")
	builder.kv(3, "sqlite", fmt.Sprintf("%t", semanticOutputObjectExists(ctx.output, "Output:SQLite")))
	builder.kv(3, "csv", fmt.Sprintf("%t", semanticOutputObjectExists(ctx.output, "OutputControl:Table:Style")))
	var variables []OutputObjectSummary
	var meters []OutputObjectSummary
	for _, item := range ctx.output.Existing {
		ctx.mark(item.ObjectIndex)
		switch strings.ToLower(item.ObjectType) {
		case "output:variable":
			variables = append(variables, item)
		case "output:meter", "output:meter:meterfileonly":
			meters = append(meters, item)
		}
	}
	writeSemanticOutputList(builder, 2, "variables", variables, true)
	writeSemanticOutputList(builder, 2, "meters", meters, true)
}

func writeSemanticAttachedOutputs(builder *semanticYAMLBuilder, indent int, key string, outputs []OutputObjectSummary) {
	writeSemanticOutputList(builder, indent, key, outputs, false)
}

func writeSemanticOutputList(builder *semanticYAMLBuilder, indent int, key string, outputs []OutputObjectSummary, includeTarget bool) {
	if len(outputs) == 0 {
		return
	}
	builder.raw(indent, key+":")
	for _, output := range outputs {
		builder.rawForObject(indent+1, "- "+yamlScalar(semanticOutputLabel(output, includeTarget)), output.ObjectIndex, output.ObjectType, output.ObjectName)
	}
}

func writeSemanticSourceNameConflicts(builder *semanticYAMLBuilder, duplicates []SemanticDuplicateGroup) {
	if len(duplicates) == 0 {
		builder.raw(1, "source_name_conflicts: []")
		return
	}
	builder.raw(1, "source_name_conflicts:")
	for _, group := range duplicates {
		builder.raw(2, "- group: "+yamlScalar(group.Group))
		builder.kv(3, "object_type", group.ObjectType)
		builder.kv(3, "name", group.Name)
		builder.raw(3, "object_indexes:")
		for _, index := range group.ObjectIndexes {
			builder.raw(4, "- "+fmt.Sprintf("%d", index))
		}
		builder.kv(3, "suggested_action", "rename_later_duplicates")
	}
}

func writeSemanticMiscellaneous(builder *semanticYAMLBuilder, ctx *semanticContext) {
	var unmapped []Object
	for _, obj := range ctx.doc.Objects {
		if !ctx.mapped[obj.Index] {
			unmapped = append(unmapped, obj)
		}
	}
	if len(unmapped) == 0 {
		builder.raw(1, "miscellaneous:")
		builder.raw(2, "other: []")
		return
	}
	builder.raw(1, "miscellaneous:")
	builder.raw(2, "other:")
	for _, obj := range unmapped {
		name := objectName(obj)
		builder.objectKV(3, "- class", obj.Type, obj.Index, obj.Type, name)
		if name != "" {
			builder.fieldKV(4, "name", name, obj.Index, obj.Type, name, 0)
		}
		builder.kvForObject(4, "reason", "unmapped_object_type", obj.Index, obj.Type, name)
		builder.rawForObject(4, "source:", obj.Index, obj.Type, name)
		builder.kvForObject(5, "object_index", fmt.Sprintf("%d", obj.Index), obj.Index, obj.Type, name)
		builder.kvForObject(5, "object_type", obj.Type, obj.Index, obj.Type, name)
		builder.rawForObject(4, "fields:", obj.Index, obj.Type, name)
		for fieldIndex, field := range obj.Fields {
			if fieldIndex == 0 && name != "" {
				continue
			}
			builder.fieldKV(5, semanticFieldKey(field, fieldIndex), field.Value, obj.Index, obj.Type, name, fieldIndex)
		}
		builder.kvForObject(4, "export_policy", "preserve_exactly", obj.Index, obj.Type, name)
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
	builder.raw(1, "source_preservation:")
	builder.kv(2, "object_order", "preserved")
	builder.kv(2, "field_order", "preserved")
	builder.kv(2, "comments", "best_effort_from_current_parser")
	builder.kv(2, "roundtrip_scope", "level_1_token_edit_patch")
	builder.kv(2, "source_registry", "internal_idf_document")
	builder.kv(2, "unmapped_policy", "miscellaneous_preserve_exactly")
}

func semanticObjectTypesWithPrefix(doc Document, prefix string) []string {
	seen := map[string]bool{}
	var out []string
	for _, obj := range doc.Objects {
		if strings.HasPrefix(strings.ToLower(obj.Type), strings.ToLower(prefix)) && !seen[strings.ToLower(obj.Type)] {
			seen[strings.ToLower(obj.Type)] = true
			out = append(out, obj.Type)
		}
	}
	sort.Strings(out)
	return out
}

func semanticObjectsForTypes(doc Document, objectTypes []string) []Object {
	wanted := map[string]bool{}
	for _, objectType := range objectTypes {
		wanted[normalizeName(objectType)] = true
	}
	var objects []Object
	for _, obj := range doc.Objects {
		if wanted[normalizeName(obj.Type)] {
			objects = append(objects, obj)
		}
	}
	return objects
}

func writeSemanticReferenceObject(builder *semanticYAMLBuilder, indent int, obj Object) {
	name := objectName(obj)
	if name != "" {
		builder.fieldKV(indent, "- name", name, obj.Index, obj.Type, name, 0)
		builder.kvForObject(indent+1, "class", obj.Type, obj.Index, obj.Type, name)
	} else {
		builder.objectKV(indent, "- class", obj.Type, obj.Index, obj.Type, name)
	}
	builder.kvForObject(indent+1, "source_object_index", fmt.Sprintf("%d", obj.Index), obj.Index, obj.Type, name)
	for fieldIndex, field := range obj.Fields {
		if fieldIndex == 0 && name != "" {
			continue
		}
		key := semanticFieldKey(field, fieldIndex)
		builder.fieldKV(indent+1, key, field.Value, obj.Index, obj.Type, name, fieldIndex)
	}
}

func semanticZones(ctx *semanticContext) []GeometryZone {
	if len(ctx.geometry.Zones) > 0 {
		return ctx.geometry.Zones
	}
	var zones []GeometryZone
	for _, obj := range ctx.doc.Objects {
		if strings.EqualFold(obj.Type, "Zone") {
			zones = append(zones, GeometryZone{ObjectIndex: obj.Index, Name: objectName(obj)})
		}
	}
	return zones
}

func semanticFieldByNames(builder *semanticYAMLBuilder, indent int, key string, obj Object, fallback string, names ...string) {
	if value, fieldIndex, ok := semanticFieldValue(obj, names...); ok {
		builder.fieldKV(indent, key, value, obj.Index, obj.Type, objectName(obj), fieldIndex)
		return
	}
	if strings.TrimSpace(fallback) != "" {
		builder.kvForObject(indent, key, fallback, obj.Index, obj.Type, objectName(obj))
	}
}

func semanticFieldValue(obj Object, names ...string) (string, int, bool) {
	if field, index, ok := fieldByCatalogName(obj, names...); ok && strings.TrimSpace(field.Value) != "" {
		return strings.TrimSpace(field.Value), index, true
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[normalizeFieldName(name)] = true
	}
	for index, field := range obj.Fields {
		if wanted[normalizeFieldName(field.Comment)] && strings.TrimSpace(field.Value) != "" {
			return strings.TrimSpace(field.Value), index, true
		}
	}
	for _, index := range semanticFallbackFieldIndexes(obj.Type, names...) {
		if index >= 0 && index < len(obj.Fields) && strings.TrimSpace(obj.Fields[index].Value) != "" {
			return strings.TrimSpace(obj.Fields[index].Value), index, true
		}
	}
	return "", -1, false
}

func semanticFallbackFieldIndexes(objectType string, names ...string) []int {
	lower := strings.ToLower(objectType)
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[normalizeFieldName(name)] = true
	}
	has := func(name string) bool { return wanted[normalizeFieldName(name)] }
	switch {
	case lower == "people":
		switch {
		case has("Zone or ZoneList Name"), has("Zone Name"):
			return []int{1}
		case has("Number of People Schedule Name"), has("Schedule Name"):
			return []int{2}
		case has("Number of People"), has("People"):
			return []int{4}
		case has("Activity Level Schedule Name"):
			return []int{6}
		}
	case lower == "lights":
		switch {
		case has("Zone or ZoneList Name"), has("Zone Name"):
			return []int{1}
		case has("Schedule Name"):
			return []int{2}
		case has("Lighting Level"), has("Design Level"):
			return []int{4}
		}
	case lower == "electricequipment" || lower == "gasequipment":
		switch {
		case has("Zone or ZoneList Name"), has("Zone Name"):
			return []int{1}
		case has("Schedule Name"):
			return []int{2}
		case has("Design Level"):
			return []int{4}
		}
	case strings.HasPrefix(lower, "zoneinfiltration:") || strings.HasPrefix(lower, "zoneventilation:") || strings.HasPrefix(lower, "zonemixing"):
		switch {
		case has("Zone or ZoneList Name"), has("Zone Name"):
			return []int{1}
		case has("Schedule Name"):
			return []int{2}
		case has("Design Flow Rate"):
			return []int{4}
		}
	case lower == "zonecontrol:thermostat":
		switch {
		case has("Zone Name"):
			return []int{1}
		case has("Control Type Schedule Name"), has("Schedule Name"):
			return []int{2}
		}
	}
	return nil
}

func semanticZoneAttachment(obj Object) (string, string, bool) {
	bucket := ""
	switch lower := strings.ToLower(obj.Type); {
	case lower == "people":
		bucket = "people"
	case lower == "lights":
		bucket = "lights"
	case lower == "electricequipment":
		bucket = "electric_equipment"
	case lower == "gasequipment":
		bucket = "gas_equipment"
	case strings.HasPrefix(lower, "zoneinfiltration:"):
		bucket = "infiltration"
	case strings.HasPrefix(lower, "zoneventilation:"):
		bucket = "ventilation"
	case strings.HasPrefix(lower, "zonemixing"):
		bucket = "mixing"
	default:
		return "", "", false
	}
	zoneName, _, ok := semanticFieldValue(obj, "Zone or ZoneList Name", "Zone Name")
	return zoneName, bucket, ok
}

func semanticControlZone(obj Object) string {
	if !strings.HasPrefix(strings.ToLower(obj.Type), "zonecontrol:") {
		return ""
	}
	zoneName, _, _ := semanticFieldValue(obj, "Zone Name")
	return zoneName
}

func semanticLoadLevel(obj Object) (string, int, bool) {
	switch strings.ToLower(obj.Type) {
	case "people":
		value, index, ok := semanticFieldValue(obj, "Number of People", "People")
		if ok {
			return strings.TrimSpace(value + " persons"), index, true
		}
	case "lights":
		value, index, ok := semanticFieldValue(obj, "Lighting Level", "Design Level")
		if ok {
			return strings.TrimSpace(value + " W"), index, true
		}
	case "electricequipment", "gasequipment":
		value, index, ok := semanticFieldValue(obj, "Design Level")
		if ok {
			return strings.TrimSpace(value + " W"), index, true
		}
	default:
		value, index, ok := semanticFieldValue(obj, "Design Flow Rate")
		if ok {
			return strings.TrimSpace(value + " m3/s"), index, true
		}
	}
	return "", -1, false
}

func semanticVertices(points []GeometryPoint) string {
	if len(points) == 0 {
		return "[]"
	}
	values := make([]string, 0, len(points))
	for _, point := range points {
		values = append(values, fmt.Sprintf("[%s,%s,%s]", semanticNumber(point.X), semanticNumber(point.Y), semanticNumber(point.Z)))
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func semanticNumber(value float64) string {
	text := fmt.Sprintf("%.4f", value)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "-0" || text == "" {
		return "0"
	}
	return text
}

func semanticQuantity(value float64, unit string) string {
	return strings.TrimSpace(semanticNumber(value) + " " + unit)
}

func semanticOutputObjectExists(report OutputReport, objectType string) bool {
	for _, item := range report.Existing {
		if strings.EqualFold(item.ObjectType, objectType) {
			return true
		}
	}
	return false
}

func semanticOutputLabel(output OutputObjectSummary, includeTarget bool) string {
	frequency := blankAs(output.ReportingFrequency, defaultOutputFrequency)
	switch strings.ToLower(output.ObjectType) {
	case "output:variable":
		if includeTarget && strings.TrimSpace(output.KeyValue) != "" && strings.TrimSpace(output.KeyValue) != "*" {
			return fmt.Sprintf("[%s] %s :: %s", frequency, output.KeyValue, output.VariableName)
		}
		return fmt.Sprintf("[%s] %s", frequency, output.VariableName)
	case "output:meter", "output:meter:meterfileonly":
		return fmt.Sprintf("[%s] %s", frequency, output.KeyValue)
	default:
		return output.Summary
	}
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
			SyncPolicy:    "rename_later_duplicates",
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
		Key:         strings.TrimPrefix(key, "- "),
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
	if lower == "true" || lower == "false" {
		return lower
	}
	if lower == "yes" || lower == "no" || lower == "on" || lower == "off" || lower == "null" {
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
