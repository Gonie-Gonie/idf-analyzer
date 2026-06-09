package idf

import (
	"fmt"
	"sort"
	"strings"
)

type semanticYAMLBuilder struct {
	model       *SemanticModel
	ctx         *semanticContext
	occurrences map[int][]SemanticOccurrence
}

func buildSemanticProjectionNodes(builder *semanticYAMLBuilder, ctx *semanticContext, metadata SemanticYAMLMetadata) {
	if builder.occurrences == nil {
		builder.occurrences = map[int][]SemanticOccurrence{}
	}
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
	builder.kv(2, "object_count", fmt.Sprintf("%d", len(ctx.doc.Objects)))
	builder.kv(2, "semantic_policy", "semantic_view_over_idf_object_registry")

	writeSemanticObjectLibrary(builder, ctx, "simulation", []string{"Version", "SimulationControl", "Timestep"})
	writeSemanticObjectLibrary(builder, ctx, "site", []string{"Site:Location", "SizingPeriod:DesignDay", "SizingPeriod:WeatherFileDays", "SizingPeriod:WeatherFileConditionType", "RunPeriod"})
	writeSemanticObjectLibrary(builder, ctx, "building", []string{"Building", "GlobalGeometryRules"})
	writeSemanticSchedules(builder, ctx)
	writeSemanticMaterials(builder, ctx)
	writeSemanticConstructions(builder, ctx)
	writeSemanticZoneGroups(builder, ctx)
	writeSemanticZones(builder, ctx)
	writeSemanticHVAC(builder, ctx)
	writeSemanticOutputs(builder, ctx)
	writeSemanticSourceNameConflicts(builder, builder.model.Source.NameConflicts)
	writeSemanticMiscellaneous(builder, ctx)
	writeSemanticSourcePreservation(builder, ctx)
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
	wildcardOutputs  []OutputObjectSummary
	scheduleRefs     map[string][]string
	zoneLists        map[string][]string
	shownFields      map[int]map[int]bool
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
		scheduleRefs:     map[string][]string{},
		zoneLists:        map[string][]string{},
		shownFields:      map[int]map[int]bool{},
	}
	for _, obj := range doc.Objects {
		ctx.objectByIndex[obj.Index] = obj
		if strings.EqualFold(obj.Type, "ZoneList") || strings.EqualFold(obj.Type, "SpaceList") {
			if name := objectName(obj); name != "" {
				ctx.zoneLists[normalizeName(name)] = semanticListMembers(obj)
				ctx.mark(obj.Index)
			}
		}
	}
	for _, surface := range ctx.geometry.Surfaces {
		ctx.surfacesByZone[normalizeName(surface.ZoneName)] = append(ctx.surfacesByZone[normalizeName(surface.ZoneName)], surface)
	}
	for _, window := range ctx.geometry.Windows {
		ctx.windowsBySurface[normalizeName(window.BaseSurfaceName)] = append(ctx.windowsBySurface[normalizeName(window.BaseSurfaceName)], window)
	}
	for _, summary := range ctx.output.Existing {
		key := normalizeName(summary.KeyValue)
		if key == "*" {
			ctx.wildcardOutputs = append(ctx.wildcardOutputs, summary)
		} else if key != "" {
			ctx.outputsByTarget[key] = append(ctx.outputsByTarget[key], summary)
		}
	}
	for _, obj := range doc.Objects {
		for _, scheduleName := range semanticScheduleReferences(obj) {
			ctx.scheduleRefs[normalizeName(scheduleName)] = append(ctx.scheduleRefs[normalizeName(scheduleName)], semanticObjectReferencePath(obj))
		}
		zoneName, bucket, ok := semanticZoneAttachment(obj)
		if ok {
			for _, target := range semanticZoneTargets(ctx, zoneName) {
				key := normalizeName(target)
				if ctx.loadsByZone[key] == nil {
					ctx.loadsByZone[key] = map[string][]Object{}
				}
				ctx.loadsByZone[key][bucket] = append(ctx.loadsByZone[key][bucket], obj)
			}
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

type semanticCompactScheduleInterval struct {
	Time            string
	TimeFieldIndex  int
	Value           string
	ValueFieldIndex int
}

type semanticCompactScheduleRule struct {
	Through           string
	ThroughFieldIndex int
	DaySelector       string
	DayFieldIndex     int
	Intervals         []semanticCompactScheduleInterval
}

func writeSemanticSchedules(builder *semanticYAMLBuilder, ctx *semanticContext) {
	objects := semanticObjectsForTypes(ctx.doc, semanticObjectTypesWithPrefix(ctx.doc, "Schedule:"))
	if len(objects) == 0 {
		builder.raw(1, "schedules: {}")
		return
	}
	builder.raw(1, "schedules:")
	for _, obj := range objects {
		ctx.mark(obj.Index)
		switch {
		case strings.EqualFold(obj.Type, "Schedule:Constant"):
			writeSemanticConstantSchedule(builder, ctx, 2, obj)
		case strings.EqualFold(obj.Type, "Schedule:Compact"):
			if rules, ok := semanticCompactScheduleRules(obj); ok {
				writeSemanticCompactSchedule(builder, ctx, 2, obj, rules)
			} else {
				writeSemanticReferenceObject(builder, 2, obj)
			}
		default:
			writeSemanticReferenceObject(builder, 2, obj)
		}
	}
}

func writeSemanticScheduleHeader(builder *semanticYAMLBuilder, ctx *semanticContext, indent int, obj Object) string {
	name := objectName(obj)
	if name != "" {
		builder.fieldKV(indent, "- name", name, obj.Index, obj.Type, name, 0)
		builder.kvForObject(indent+1, "class", obj.Type, obj.Index, obj.Type, name)
	} else {
		builder.objectKV(indent, "- class", obj.Type, obj.Index, obj.Type, name)
	}
	builder.kvForObject(indent+1, "source_object_index", fmt.Sprintf("%d", obj.Index), obj.Index, obj.Type, name)
	if value, fieldIndex, ok := semanticScheduleField(obj, 1, "Schedule Type Limits Name", "Schedule Type Limits"); ok {
		builder.fieldKV(indent+1, "type_limits", value, obj.Index, obj.Type, name, fieldIndex)
	}
	if hours, ok := annualScheduleHours(obj); ok {
		builder.kvForObject(indent+1, "active_hours_per_year", semanticNumber(hours), obj.Index, obj.Type, name)
	}
	usedBy := sortedUniqueStrings(ctx.scheduleRefs[normalizeName(name)])
	if len(usedBy) > 0 {
		builder.rawForObject(indent+1, "used_by:", obj.Index, obj.Type, name)
		for _, path := range usedBy {
			builder.rawForObject(indent+2, "- "+yamlScalar(path), obj.Index, obj.Type, name)
		}
	}
	return name
}

func writeSemanticConstantSchedule(builder *semanticYAMLBuilder, ctx *semanticContext, indent int, obj Object) {
	name := writeSemanticScheduleHeader(builder, ctx, indent, obj)
	if value, fieldIndex, ok := semanticScheduleField(obj, 2, "Hourly Value"); ok {
		builder.fieldKV(indent+1, "default", value, obj.Index, obj.Type, name, fieldIndex)
	}
}

func writeSemanticCompactSchedule(builder *semanticYAMLBuilder, ctx *semanticContext, indent int, obj Object, rules []semanticCompactScheduleRule) {
	name := writeSemanticScheduleHeader(builder, ctx, indent, obj)
	builder.rawForObject(indent+1, "rules:", obj.Index, obj.Type, name)
	for _, rule := range rules {
		if rule.Through != "" && rule.ThroughFieldIndex >= 0 {
			builder.fieldKV(indent+2, "- through", rule.Through, obj.Index, obj.Type, name, rule.ThroughFieldIndex)
		} else {
			builder.objectKV(indent+2, "- through", "unspecified", obj.Index, obj.Type, name)
		}
		if rule.DaySelector != "" && rule.DayFieldIndex >= 0 {
			builder.fieldKV(indent+3, "for", rule.DaySelector, obj.Index, obj.Type, name, rule.DayFieldIndex)
		}
		if len(rule.Intervals) == 0 {
			continue
		}
		builder.rawForObject(indent+3, "until:", obj.Index, obj.Type, name)
		for _, interval := range rule.Intervals {
			builder.fieldKV(indent+4, "- time", interval.Time, obj.Index, obj.Type, name, interval.TimeFieldIndex)
			builder.fieldKV(indent+5, "value", interval.Value, obj.Index, obj.Type, name, interval.ValueFieldIndex)
		}
	}
}

func semanticScheduleField(obj Object, fallbackIndex int, names ...string) (string, int, bool) {
	if value, fieldIndex, ok := semanticFieldValue(obj, names...); ok {
		return value, fieldIndex, true
	}
	if fallbackIndex >= 0 && fallbackIndex < len(obj.Fields) {
		value := strings.TrimSpace(obj.Fields[fallbackIndex].Value)
		if value != "" {
			return value, fallbackIndex, true
		}
	}
	return "", -1, false
}

func semanticCompactScheduleRules(obj Object) ([]semanticCompactScheduleRule, bool) {
	if len(obj.Fields) <= 2 {
		return nil, false
	}
	var rules []semanticCompactScheduleRule
	through := ""
	throughFieldIndex := -1
	for fieldIndex := 2; fieldIndex < len(obj.Fields); {
		directive, value, ok := semanticCompactScheduleDirective(obj.Fields[fieldIndex].Value)
		if !ok {
			fieldIndex++
			continue
		}
		switch directive {
		case "through":
			through = value
			throughFieldIndex = fieldIndex
			fieldIndex++
		case "for":
			rule := semanticCompactScheduleRule{
				Through:           through,
				ThroughFieldIndex: throughFieldIndex,
				DaySelector:       value,
				DayFieldIndex:     fieldIndex,
			}
			fieldIndex++
			for fieldIndex < len(obj.Fields) {
				nextDirective, nextValue, nextOK := semanticCompactScheduleDirective(obj.Fields[fieldIndex].Value)
				if nextOK && (nextDirective == "through" || nextDirective == "for") {
					break
				}
				if !nextOK || nextDirective != "until" {
					fieldIndex++
					continue
				}
				if fieldIndex+1 >= len(obj.Fields) {
					return nil, false
				}
				rule.Intervals = append(rule.Intervals, semanticCompactScheduleInterval{
					Time:            nextValue,
					TimeFieldIndex:  fieldIndex,
					Value:           strings.TrimSpace(obj.Fields[fieldIndex+1].Value),
					ValueFieldIndex: fieldIndex + 1,
				})
				fieldIndex += 2
			}
			rules = append(rules, rule)
		default:
			fieldIndex++
		}
	}
	return rules, len(rules) > 0
}

func semanticCompactScheduleDirective(value string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	directive := strings.ToLower(strings.TrimSpace(parts[0]))
	switch directive {
	case "through", "for", "until":
		return directive, strings.TrimSpace(parts[1]), true
	default:
		return "", "", false
	}
}

func writeSemanticMaterials(builder *semanticYAMLBuilder, ctx *semanticContext) {
	materials := semanticMaterialObjects(ctx.doc)
	if len(materials) == 0 {
		builder.raw(1, "materials: {}")
		return
	}
	builder.raw(1, "materials:")
	for _, obj := range materials {
		ctx.mark(obj.Index)
		name := objectName(obj)
		builder.fieldKV(2, "- name", name, obj.Index, obj.Type, name, 0)
		builder.kvForObject(3, "class", obj.Type, obj.Index, obj.Type, name)
		builder.kvForObject(3, "source_object_index", fmt.Sprintf("%d", obj.Index), obj.Index, obj.Type, name)
		for _, fieldName := range []string{"Roughness", "Thickness", "Conductivity", "Density", "Specific Heat", "Thermal Resistance", "U-Factor"} {
			key := semanticFieldKeyFromName(fieldName)
			semanticFieldByNames(builder, 3, key, obj, "", fieldName)
		}
		usedBy := semanticMaterialUsedBy(ctx.doc, name)
		if len(usedBy) > 0 {
			builder.rawForObject(3, "used_by:", obj.Index, obj.Type, name)
			for _, item := range usedBy {
				builder.rawForObject(4, "- "+yamlScalar(item), obj.Index, obj.Type, name)
			}
		}
	}
}

func writeSemanticConstructions(builder *semanticYAMLBuilder, ctx *semanticContext) {
	if len(ctx.geometry.Constructions) == 0 && len(semanticFenestrationConstructionObjects(ctx.doc)) == 0 {
		builder.raw(1, "constructions: {}")
		return
	}
	builder.raw(1, "constructions:")
	builder.raw(2, "opaque:")
	if len(ctx.geometry.Constructions) == 0 {
		builder.raw(3, "[]")
	}
	for _, construction := range ctx.geometry.Constructions {
		ctx.mark(construction.ObjectIndex)
		name := construction.Name
		builder.fieldKV(3, "- name", name, construction.ObjectIndex, construction.ObjectType, name, 0)
		builder.kvForObject(4, "class", construction.ObjectType, construction.ObjectIndex, construction.ObjectType, name)
		builder.kvForObject(4, "source_object_index", fmt.Sprintf("%d", construction.ObjectIndex), construction.ObjectIndex, construction.ObjectType, name)
		usedBy := semanticConstructionUsedBy(ctx, name)
		if len(usedBy) > 0 {
			builder.rawForObject(4, "used_by:", construction.ObjectIndex, construction.ObjectType, name)
			for _, item := range usedBy {
				builder.rawForObject(5, "- "+yamlScalar(item), construction.ObjectIndex, construction.ObjectType, name)
			}
		}
		if len(construction.Layers) > 0 {
			builder.rawForObject(4, "layers:", construction.ObjectIndex, construction.ObjectType, name)
			for layerIndex, layer := range construction.Layers {
				if layer.ObjectIndex >= 0 {
					ctx.mark(layer.ObjectIndex)
				}
				layerName := blankAs(layer.Name, "unnamed_layer")
				builder.rawForObject(5, "- name: "+yamlScalar(layerName), construction.ObjectIndex, construction.ObjectType, name)
				builder.kvForObject(6, "order", fmt.Sprintf("%d", layerIndex+1), construction.ObjectIndex, construction.ObjectType, name)
				if layer.ObjectType != "" {
					builder.kvForObject(6, "class", layer.ObjectType, construction.ObjectIndex, construction.ObjectType, name)
				}
				if layer.HasThickness {
					builder.kvForObject(6, "thickness", semanticQuantity(layer.Thickness, "m"), construction.ObjectIndex, construction.ObjectType, name)
				}
			}
		}
	}
	fenestration := semanticFenestrationConstructionObjects(ctx.doc)
	if len(fenestration) == 0 {
		builder.raw(2, "fenestration: []")
		return
	}
	builder.raw(2, "fenestration:")
	for _, obj := range fenestration {
		ctx.mark(obj.Index)
		writeSemanticReferenceObject(builder, 3, obj)
	}
}

func writeSemanticZoneGroups(builder *semanticYAMLBuilder, ctx *semanticContext) {
	var groups []Object
	for _, obj := range ctx.doc.Objects {
		if strings.EqualFold(obj.Type, "ZoneList") || strings.EqualFold(obj.Type, "SpaceList") {
			groups = append(groups, obj)
		}
	}
	if len(groups) == 0 {
		builder.raw(1, "zone_groups: []")
		return
	}
	builder.raw(1, "zone_groups:")
	for _, obj := range groups {
		ctx.mark(obj.Index)
		name := objectName(obj)
		builder.fieldKV(2, "- name", name, obj.Index, obj.Type, name, 0)
		builder.kvForObject(3, "class", obj.Type, obj.Index, obj.Type, name)
		builder.rawForObject(3, "members:", obj.Index, obj.Type, name)
		for index := 1; index < len(obj.Fields); index++ {
			value := strings.TrimSpace(obj.Fields[index].Value)
			if value != "" {
				builder.fieldKV(4, "- name", value, obj.Index, obj.Type, name, index)
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
		writeSemanticAttachedOutputs(builder, 3, "inherited_outputs", ctx.wildcardOutputs)
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
		semanticFieldByNames(builder, 7, "object", ctx.objectByIndex[surface.ObjectIndex], "", "Outside Boundary Condition Object")
		builder.rawForObject(6, "exposure:", surface.ObjectIndex, surface.Type, surface.Name)
		semanticFieldByNames(builder, 7, "sun", ctx.objectByIndex[surface.ObjectIndex], "", "Sun Exposure")
		semanticFieldByNames(builder, 7, "wind", ctx.objectByIndex[surface.ObjectIndex], "", "Wind Exposure")
		semanticFieldByNames(builder, 6, "view_factor_to_ground", ctx.objectByIndex[surface.ObjectIndex], "", "View Factor to Ground")
		builder.rawForObject(6, "vertices: "+semanticVertices(surface.Vertices), surface.ObjectIndex, surface.Type, surface.Name)
		builder.rawForObject(6, "computed:", surface.ObjectIndex, surface.Type, surface.Name)
		builder.kvForObject(7, "area", semanticQuantity(surface.Area, "m2"), surface.ObjectIndex, surface.Type, surface.Name)
		if surface.Orientation != "" {
			builder.kvForObject(7, "orientation", surface.Orientation, surface.ObjectIndex, surface.Type, surface.Name)
		}
		builder.kvForObject(7, "azimuth", semanticQuantity(surface.Azimuth, "deg"), surface.ObjectIndex, surface.Type, surface.Name)
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
				builder.rawForObject(8, "computed:", window.ObjectIndex, window.Type, window.Name)
				builder.kvForObject(9, "area", semanticQuantity(window.Area, "m2"), window.ObjectIndex, window.Type, window.Name)
				if window.Orientation != "" {
					builder.kvForObject(9, "orientation", window.Orientation, window.ObjectIndex, window.Type, window.Name)
				}
				builder.kvForObject(9, "azimuth", semanticQuantity(window.Azimuth, "deg"), window.ObjectIndex, window.Type, window.Name)
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
	writeSemanticZoneObjectBuckets(builder, ctx, 3, "loads", []string{"people", "lights", "electric_equipment", "gas_equipment"}, buckets)
	writeSemanticZoneObjectBuckets(builder, ctx, 3, "air_exchange", []string{"infiltration", "ventilation", "mixing", "cross_mixing"}, buckets)
}

func writeSemanticZoneObjectBuckets(builder *semanticYAMLBuilder, ctx *semanticContext, indent int, section string, order []string, buckets map[string][]Object) {
	hasAny := false
	for _, bucket := range order {
		if len(buckets[bucket]) > 0 {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return
	}
	builder.raw(indent, section+":")
	for _, bucket := range order {
		objects := buckets[bucket]
		if len(objects) == 0 {
			continue
		}
		builder.raw(indent+1, bucket+":")
		for _, obj := range objects {
			ctx.mark(obj.Index)
			writeSemanticLoadObject(builder, indent+2, obj)
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
	if displayValue, sourceValue, fieldIndex, ok := semanticLoadLevel(obj); ok {
		builder.fieldDisplayKV(indent+1, "level", displayValue, sourceValue, obj.Index, obj.Type, name, fieldIndex)
	}
	if target, _, ok := semanticZoneAttachment(obj); ok {
		if members := builder.ctx.zoneLists[normalizeName(target)]; len(members) > 1 {
			writeSemanticDuplicatedAs(builder, indent+1, obj.Index, obj.Type, name, "zone_group_expanded_load", semanticZoneGroupOccurrencePaths(members, obj))
		}
	}
	writeSemanticAttachedOutputs(builder, indent+1, "outputs", builder.ctx.outputsByTarget[normalizeName(name)])
}

func writeSemanticZoneControls(builder *semanticYAMLBuilder, ctx *semanticContext, zoneName string) {
	controls := ctx.controlsByZone[normalizeName(zoneName)]
	if len(controls) == 0 {
		return
	}
	builder.raw(3, "controls:")
	buckets := map[string][]Object{}
	order := []string{"thermostat", "daylighting", "humidistat", "other"}
	for _, obj := range controls {
		bucket := semanticControlBucket(obj.Type)
		buckets[bucket] = append(buckets[bucket], obj)
	}
	for _, bucket := range order {
		objects := buckets[bucket]
		if len(objects) == 0 {
			continue
		}
		builder.raw(4, bucket+":")
		for _, obj := range objects {
			ctx.mark(obj.Index)
			name := objectName(obj)
			builder.fieldKV(5, "- name", name, obj.Index, obj.Type, name, 0)
			builder.kvForObject(6, "class", obj.Type, obj.Index, obj.Type, name)
			if value, fieldIndex, ok := semanticFieldValue(obj, "Zone Name"); ok {
				builder.fieldKV(6, "zone", value, obj.Index, obj.Type, name, fieldIndex)
			}
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
	writeSemanticHVACLoops(builder, ctx, "air_loops", "AirLoopHVAC")
	writeSemanticHVACLoops(builder, ctx, "plant_loops", "PlantLoop")
	writeSemanticHVACEquipmentCatalog(builder, ctx)
	writeSemanticHVACNodes(builder, ctx)
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
			if branch.InletNode != "" {
				builder.kvForObject(indent+3, "inlet_node", branch.InletNode, branch.ObjectIndex, "Branch", branch.Name)
			}
			if branch.OutletNode != "" {
				builder.kvForObject(indent+3, "outlet_node", branch.OutletNode, branch.ObjectIndex, "Branch", branch.Name)
			}
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
	if component.WaterInletNode != "" {
		builder.kvForObject(indent+1, "water_inlet_node", component.WaterInletNode, objectIndex, component.ObjectType, name)
	}
	if component.WaterOutletNode != "" {
		builder.kvForObject(indent+1, "water_outlet_node", component.WaterOutletNode, objectIndex, component.ObjectType, name)
	}
	if !component.Exists {
		builder.kvForObject(indent+1, "exists", "false", objectIndex, component.ObjectType, name)
		builder.kvForObject(indent+1, "reason", "unresolved_component_reference", objectIndex, component.ObjectType, name)
	}
	if objectIndex >= 0 {
		writeSemanticDuplicatedAs(builder, indent+1, objectIndex, component.ObjectType, name, role, []string{"hvac/equipment_catalog/" + blankAs(name, component.ObjectType)})
	}
}

func writeSemanticDuplicatedAs(builder *semanticYAMLBuilder, indent int, objectIndex int, objectType string, objectName string, role string, alsoShownIn []string) {
	builder.rawForObject(indent, "duplicated_as:", objectIndex, objectType, objectName)
	builder.kvForObject(indent+1, "group", "obj-"+fmt.Sprintf("%d", objectIndex), objectIndex, objectType, objectName)
	builder.kvForObject(indent+1, "role_here", role, objectIndex, objectType, objectName)
	builder.rawForObject(indent+1, "also_shown_in:", objectIndex, objectType, objectName)
	for _, path := range sortedUniqueStrings(alsoShownIn) {
		builder.rawForObject(indent+2, "- "+yamlScalar(path), objectIndex, objectType, objectName)
	}
	builder.kvForObject(indent+1, "sync_policy", "edit_once_sync_all", objectIndex, objectType, objectName)
}

func semanticZoneGroupOccurrencePaths(members []string, obj Object) []string {
	_, bucket, _ := semanticZoneAttachment(obj)
	section := "loads"
	if semanticAirExchangeBucket(bucket) {
		section = "air_exchange"
	}
	name := blankAs(objectName(obj), obj.Type)
	paths := make([]string, 0, len(members))
	for _, zone := range members {
		paths = append(paths, "zones/"+zone+"/"+section+"/"+bucket+"/"+name)
	}
	return paths
}

func writeSemanticHVACEquipmentCatalog(builder *semanticYAMLBuilder, ctx *semanticContext) {
	seen := map[int]HVACComponent{}
	for _, loop := range ctx.hvac.Loops {
		for _, component := range loopComponents(loop) {
			if component.ObjectIndex >= 0 && component.Exists {
				seen[component.ObjectIndex] = component
			}
		}
	}
	for _, relation := range ctx.hvac.ZoneRelations {
		for _, component := range append(append([]HVACComponent{}, relation.TerminalUnits...), append(relation.ZoneEquipment, relation.PlantEquipment...)...) {
			if component.ObjectIndex >= 0 && component.Exists {
				seen[component.ObjectIndex] = component
			}
		}
	}
	if len(seen) == 0 {
		return
	}
	indexes := make([]int, 0, len(seen))
	for index := range seen {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	builder.raw(2, "equipment_catalog:")
	for _, index := range indexes {
		component := seen[index]
		ctx.mark(component.ObjectIndex)
		writeSemanticHVACComponent(builder, 3, component, "equipment_catalog")
	}
}

func writeSemanticHVACNodes(builder *semanticYAMLBuilder, ctx *semanticContext) {
	if len(ctx.hvac.NodeUsages) == 0 {
		return
	}
	builder.raw(2, "nodes:")
	byName := map[string][]HVACNodeUsage{}
	for _, usage := range ctx.hvac.NodeUsages {
		byName[normalizeName(usage.NodeName)] = append(byName[normalizeName(usage.NodeName)], usage)
	}
	keys := make([]string, 0, len(byName))
	for key := range byName {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		usages := byName[key]
		if len(usages) == 0 {
			continue
		}
		nodeName := usages[0].NodeName
		builder.raw(3, "- name: "+yamlScalar(nodeName))
		builder.raw(4, "used_by:")
		for _, usage := range usages {
			label := usage.ObjectType
			if usage.ObjectName != "" {
				label += " " + usage.ObjectName
			}
			label += " / " + usage.Role
			builder.rawForObject(5, "- "+yamlScalar(label), usage.ObjectIndex, usage.ObjectType, usage.ObjectName)
		}
	}
}

func writeSemanticOutputs(builder *semanticYAMLBuilder, ctx *semanticContext) {
	builder.raw(1, "outputs:")
	builder.raw(2, "files:")
	writeSemanticOutputFileStatus(builder, 3, "csv", "OutputControl:Files", semanticOutputControlFilesEnabled(ctx.doc, "csv"))
	writeSemanticOutputFileStatus(builder, 3, "sqlite", "Output:SQLite", semanticOutputObjectExists(ctx.output, "Output:SQLite") || semanticOutputControlFilesEnabled(ctx.doc, "sqlite"))
	writeSemanticOutputFileStatus(builder, 3, "json", "Output:JSON", semanticOutputObjectExists(ctx.output, "Output:JSON") || semanticOutputControlFilesEnabled(ctx.doc, "json"))
	var variables []OutputObjectSummary
	var meters []OutputObjectSummary
	var summaryReports []OutputObjectSummary
	var tableStyles []OutputObjectSummary
	var wildcard []OutputObjectSummary
	var unresolved []OutputObjectSummary
	for _, item := range ctx.output.Existing {
		ctx.mark(item.ObjectIndex)
		switch strings.ToLower(item.ObjectType) {
		case "output:variable":
			variables = append(variables, item)
			if strings.TrimSpace(item.KeyValue) == "*" {
				wildcard = append(wildcard, item)
			} else if strings.TrimSpace(item.KeyValue) != "" && !semanticOutputTargetExists(ctx, item.KeyValue) {
				unresolved = append(unresolved, item)
			}
		case "output:meter", "output:meter:meterfileonly":
			meters = append(meters, item)
		case "output:table:summaryreports":
			summaryReports = append(summaryReports, item)
		case "outputcontrol:table:style":
			tableStyles = append(tableStyles, item)
		}
	}
	writeSemanticOutputList(builder, 2, "variables", variables, true)
	writeSemanticOutputList(builder, 2, "meters", meters, true)
	writeSemanticOutputList(builder, 2, "wildcard", wildcard, true)
	writeSemanticOutputList(builder, 2, "unresolved", unresolved, true)
	if len(summaryReports) > 0 || len(tableStyles) > 0 {
		builder.raw(2, "tabular:")
		writeSemanticOutputList(builder, 3, "summary_reports", summaryReports, false)
		if len(tableStyles) > 0 {
			builder.raw(3, "style:")
			for _, item := range tableStyles {
				builder.rawForObject(4, "- source: "+yamlScalar(item.ObjectType), item.ObjectIndex, item.ObjectType, item.ObjectName)
				for _, field := range item.Fields {
					if strings.TrimSpace(field.Value) != "" {
						builder.fieldKV(5, semanticFieldKeyFromName(field.Name), field.Value, item.ObjectIndex, item.ObjectType, item.ObjectName, field.Index)
					}
				}
			}
		}
	}
	writeSemanticHeatFlowOutputGroup(builder, 2, variables)
}

func writeSemanticOutputFileStatus(builder *semanticYAMLBuilder, indent int, key string, source string, enabled bool) {
	builder.raw(indent, key+":")
	builder.kv(indent+1, "enabled", fmt.Sprintf("%t", enabled))
	builder.kv(indent+1, "source", source)
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
		label := semanticOutputLabel(output, includeTarget)
		switch key {
		case "wildcard":
			builder.rawForObject(indent+1, "- request: "+yamlScalar(label), output.ObjectIndex, output.ObjectType, output.ObjectName)
			builder.kvForObject(indent+2, "scope", "wildcard", output.ObjectIndex, output.ObjectType, output.ObjectName)
		case "unresolved":
			builder.rawForObject(indent+1, "- request: "+yamlScalar(label), output.ObjectIndex, output.ObjectType, output.ObjectName)
			builder.kvForObject(indent+2, "reason", "target_key_not_resolved", output.ObjectIndex, output.ObjectType, output.ObjectName)
		default:
			builder.rawForObject(indent+1, "- "+yamlScalar(label), output.ObjectIndex, output.ObjectType, output.ObjectName)
		}
	}
}

func writeSemanticSourceNameConflicts(builder *semanticYAMLBuilder, duplicates []SemanticSourceNameConflict) {
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
		builder.kvForObject(4, "reason", semanticMiscReason(obj.Type), obj.Index, obj.Type, name)
		if suggested := semanticSuggestedSection(obj.Type); suggested != "" {
			builder.kvForObject(4, "suggested_section", suggested, obj.Index, obj.Type, name)
		}
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

func writeSemanticSourcePreservation(builder *semanticYAMLBuilder, ctx *semanticContext) {
	builder.raw(1, "source_preservation:")
	builder.kv(2, "object_order", "preserved")
	builder.kv(2, "field_order", "preserved")
	builder.kv(2, "comments", "best_effort_from_current_parser")
	builder.kv(2, "mode", "internal_projection")
	builder.kv(2, "editable_scope", "visible_raw_fields_only")
	builder.kv(2, "roundtrip_scope", "app_state_patch_not_standalone_yaml_import")
	builder.kv(2, "source_registry", "internal_idf_document")
	builder.kv(2, "unmapped_policy", "miscellaneous_preserve_exactly")
	entries := []Object{}
	for _, obj := range ctx.doc.Objects {
		if !ctx.mapped[obj.Index] {
			continue
		}
		if semanticUnshownFieldCount(ctx, obj.Index) > 0 {
			entries = append(entries, obj)
		}
	}
	if len(entries) == 0 {
		builder.raw(2, "mapped_object_unshown_fields: []")
		return
	}
	builder.raw(2, "mapped_object_unshown_fields:")
	for _, obj := range entries {
		count := semanticUnshownFieldCount(ctx, obj.Index)
		name := objectName(obj)
		label := obj.Type
		if name != "" {
			label += " " + name
		}
		builder.rawForObject(3, "- object: "+yamlScalar(label), obj.Index, obj.Type, name)
		builder.kvForObject(4, "source_object_index", fmt.Sprintf("%d", obj.Index), obj.Index, obj.Type, name)
		builder.kvForObject(4, "unshown_field_count", fmt.Sprintf("%d", count), obj.Index, obj.Type, name)
	}
}

func semanticUnshownFieldCount(ctx *semanticContext, objectIndex int) int {
	obj, ok := ctx.objectByIndex[objectIndex]
	if !ok {
		return 0
	}
	shown := ctx.shownFields[objectIndex]
	count := 0
	for index, field := range obj.Fields {
		if strings.TrimSpace(field.Value) == "" {
			continue
		}
		if shown[index] {
			continue
		}
		count++
	}
	return count
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

func semanticMaterialObjects(doc Document) []Object {
	var objects []Object
	for _, obj := range doc.Objects {
		if isGeometryMaterialType(obj.Type) {
			objects = append(objects, obj)
		}
	}
	return objects
}

func semanticFenestrationConstructionObjects(doc Document) []Object {
	var objects []Object
	for _, obj := range doc.Objects {
		lower := strings.ToLower(strings.TrimSpace(obj.Type))
		if strings.HasPrefix(lower, "construction:") {
			objects = append(objects, obj)
		}
	}
	return objects
}

func semanticFieldKeyFromName(name string) string {
	return semanticFieldKey(Field{Comment: name}, 0)
}

func semanticMaterialUsedBy(doc Document, materialName string) []string {
	var out []string
	key := normalizeName(materialName)
	for _, obj := range doc.Objects {
		if !strings.EqualFold(obj.Type, "Construction") || objectName(obj) == "" {
			continue
		}
		for fieldIndex := 1; fieldIndex < len(obj.Fields); fieldIndex++ {
			if normalizeName(obj.Fields[fieldIndex].Value) == key {
				out = append(out, "constructions/opaque/"+objectName(obj))
			}
		}
	}
	return sortedUniqueStrings(out)
}

func semanticConstructionUsedBy(ctx *semanticContext, constructionName string) []string {
	var out []string
	key := normalizeName(constructionName)
	for _, surface := range ctx.geometry.Surfaces {
		if normalizeName(surface.Construction) == key {
			out = append(out, "zones/"+surface.ZoneName+"/geometry/surfaces/"+surface.Name)
		}
	}
	for _, window := range ctx.geometry.Windows {
		if normalizeName(window.Construction) == key {
			out = append(out, "zones/"+window.ZoneName+"/geometry/fenestration/"+window.Name)
		}
	}
	return sortedUniqueStrings(out)
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

func semanticScheduleReferences(obj Object) []string {
	if strings.HasPrefix(strings.ToLower(obj.Type), "schedule:") {
		return nil
	}
	var refs []string
	for index, field := range obj.Fields {
		value := strings.TrimSpace(field.Value)
		if value == "" {
			continue
		}
		fieldName := catalogFieldName(obj, index)
		if fieldName == "" {
			fieldName = field.Comment
		}
		normalized := normalizeFieldName(fieldName)
		if strings.Contains(normalized, "schedule") && strings.Contains(normalized, "name") {
			refs = append(refs, value)
		}
	}
	return sortedUniqueStrings(refs)
}

func semanticObjectReferencePath(obj Object) string {
	name := objectName(obj)
	zoneName, bucket, ok := semanticZoneAttachment(obj)
	if ok {
		section := "loads"
		if semanticAirExchangeBucket(bucket) {
			section = "air_exchange"
		}
		return "zones/" + blankAs(zoneName, "unresolved_zone") + "/" + section + "/" + bucket + "/" + blankAs(name, obj.Type)
	}
	if zoneName := semanticControlZone(obj); zoneName != "" {
		return "zones/" + zoneName + "/controls/" + semanticControlBucket(obj.Type) + "/" + blankAs(name, obj.Type)
	}
	if name != "" {
		return strings.ToLower(obj.Type) + "/" + name
	}
	return strings.ToLower(obj.Type) + "/object-" + fmt.Sprintf("%d", obj.Index)
}

func semanticAirExchangeBucket(bucket string) bool {
	switch bucket {
	case "infiltration", "ventilation", "mixing", "cross_mixing":
		return true
	default:
		return false
	}
}

func semanticListMembers(obj Object) []string {
	var values []string
	for index := 1; index < len(obj.Fields); index++ {
		value := strings.TrimSpace(obj.Fields[index].Value)
		if value != "" {
			values = append(values, value)
		}
	}
	return sortedUniqueStrings(values)
}

func semanticZoneTargets(ctx *semanticContext, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	if members := ctx.zoneLists[normalizeName(target)]; len(members) > 0 {
		return members
	}
	return []string{target}
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
	case strings.HasPrefix(lower, "zonecrossmixing"):
		bucket = "cross_mixing"
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

func semanticControlBucket(objectType string) string {
	lower := strings.ToLower(strings.TrimSpace(objectType))
	switch {
	case strings.Contains(lower, "thermostat") || strings.Contains(lower, "thermostatsetpoint"):
		return "thermostat"
	case strings.Contains(lower, "daylighting"):
		return "daylighting"
	case strings.Contains(lower, "humidistat"):
		return "humidistat"
	default:
		return "other"
	}
}

func semanticLoadLevel(obj Object) (string, string, int, bool) {
	switch strings.ToLower(obj.Type) {
	case "people":
		value, index, ok := semanticFieldValue(obj, "Number of People", "People")
		if ok {
			return strings.TrimSpace(value + " persons"), value, index, true
		}
	case "lights":
		value, index, ok := semanticFieldValue(obj, "Lighting Level", "Design Level")
		if ok {
			return strings.TrimSpace(value + " W"), value, index, true
		}
	case "electricequipment", "gasequipment":
		value, index, ok := semanticFieldValue(obj, "Design Level")
		if ok {
			return strings.TrimSpace(value + " W"), value, index, true
		}
	default:
		value, index, ok := semanticFieldValue(obj, "Design Flow Rate")
		if ok {
			return strings.TrimSpace(value + " m3/s"), value, index, true
		}
	}
	return "", "", -1, false
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

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func semanticOutputObjectExists(report OutputReport, objectType string) bool {
	for _, item := range report.Existing {
		if strings.EqualFold(item.ObjectType, objectType) {
			return true
		}
	}
	return false
}

func semanticOutputControlFilesEnabled(doc Document, fileKind string) bool {
	for _, obj := range doc.Objects {
		if !strings.EqualFold(obj.Type, "OutputControl:Files") {
			continue
		}
		wanted := normalizeFieldName("Output " + fileKind)
		for index, field := range obj.Fields {
			name := normalizeFieldName(catalogFieldName(obj, index))
			if name == "" {
				name = normalizeFieldName(field.Comment)
			}
			if name != wanted && !strings.Contains(name, normalizeFieldName(fileKind)) {
				continue
			}
			return semanticYesNoValue(field.Value)
		}
	}
	return false
}

func semanticYesNoValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "1", "on":
		return true
	default:
		return false
	}
}

func semanticOutputTargetExists(ctx *semanticContext, keyValue string) bool {
	key := normalizeName(keyValue)
	if key == "" || key == "*" {
		return true
	}
	for _, obj := range ctx.doc.Objects {
		if normalizeName(objectName(obj)) == key {
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

func writeSemanticHeatFlowOutputGroup(builder *semanticYAMLBuilder, indent int, variables []OutputObjectSummary) {
	var heatFlow []OutputObjectSummary
	for _, item := range variables {
		if strings.Contains(strings.ToLower(item.VariableName), "zone air heat balance") {
			heatFlow = append(heatFlow, item)
		}
	}
	if len(heatFlow) == 0 {
		return
	}
	builder.raw(indent, "groups:")
	builder.raw(indent+1, "heat_flow_ledger:")
	builder.kv(indent+2, "frequency", standardHeatFlowFrequency)
	writeSemanticOutputList(builder, indent+2, "variables", heatFlow, false)
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

func semanticMiscReason(objectType string) string {
	section := semanticSuggestedSection(objectType)
	switch section {
	case "":
		return "unmapped_object_type"
	case "miscellaneous":
		return "raw_preservation_only"
	default:
		return "known_type_not_projected_yet"
	}
}

func semanticSuggestedSection(objectType string) string {
	section := semanticSectionForType(objectType)
	if section == "miscellaneous" {
		return ""
	}
	return section
}

func semanticSourceNameConflicts(doc Document) []SemanticSourceNameConflict {
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
	var groups []SemanticSourceNameConflict
	for _, item := range byKey {
		if len(item.indexes) < 2 {
			continue
		}
		sort.Ints(item.indexes)
		groups = append(groups, SemanticSourceNameConflict{
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
