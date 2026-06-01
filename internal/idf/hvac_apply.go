package idf

import (
	"fmt"
	"strconv"
	"strings"
)

type HVACApplyRequest struct {
	Changes []HVACFieldEditRequest `json:"changes"`
}

type HVACFieldEditRequest struct {
	ObjectIndex int    `json:"objectIndex"`
	FieldIndex  int    `json:"fieldIndex"`
	Value       string `json:"value"`
	Reason      string `json:"reason,omitempty"`
}

type HVACApplyPreview struct {
	CanApply bool              `json:"canApply"`
	Changes  []HVACApplyChange `json:"changes"`
	Warnings []HVACWarning     `json:"warnings,omitempty"`
}

type HVACApplyChange struct {
	Action       string `json:"action"`
	ObjectIndex  int    `json:"objectIndex"`
	ObjectType   string `json:"objectType,omitempty"`
	ObjectName   string `json:"objectName,omitempty"`
	FieldIndex   int    `json:"fieldIndex"`
	FieldName    string `json:"fieldName,omitempty"`
	EditKind     string `json:"editKind,omitempty"`
	Before       string `json:"before,omitempty"`
	After        string `json:"after,omitempty"`
	Message      string `json:"message"`
	RequiresSave bool   `json:"requiresSave"`
}

func PreviewApplyHVAC(doc Document, request HVACApplyRequest) HVACApplyPreview {
	_, preview := applyHVAC(doc, request, false)
	return preview
}

func ApplyHVAC(doc Document, request HVACApplyRequest) (Document, HVACApplyPreview) {
	return applyHVAC(doc, request, true)
}

func applyHVAC(doc Document, request HVACApplyRequest, mutate bool) (Document, HVACApplyPreview) {
	updated := doc.clone()
	preview := HVACApplyPreview{CanApply: true}
	if len(request.Changes) == 0 {
		preview.CanApply = false
		preview.Warnings = append(preview.Warnings, HVACWarning{
			Severity: DiagnosticWarning,
			Category: "HVAC Apply",
			Code:     "missing_hvac_changes",
			Message:  "No HVAC changes were selected.",
		})
		return updated, preview
	}

	seen := map[string]bool{}
	for _, change := range request.Changes {
		key := fmt.Sprintf("%d:%d", change.ObjectIndex, change.FieldIndex)
		if seen[key] {
			preview.CanApply = false
			preview.Warnings = append(preview.Warnings, HVACWarning{
				Severity: DiagnosticError,
				Category: "HVAC Apply",
				Code:     "duplicate_hvac_change",
				Message:  fmt.Sprintf("Field %s was selected more than once.", key),
			})
			continue
		}
		seen[key] = true
		applyHVACFieldChange(&updated, doc, change, mutate, &preview)
	}
	return updated, preview
}

func applyHVACFieldChange(updated *Document, original Document, request HVACFieldEditRequest, mutate bool, preview *HVACApplyPreview) {
	if request.ObjectIndex < 0 || request.ObjectIndex >= len(original.Objects) {
		preview.CanApply = false
		preview.Warnings = append(preview.Warnings, HVACWarning{
			Severity: DiagnosticError,
			Category: "HVAC Apply",
			Code:     "hvac_object_out_of_range",
			Message:  fmt.Sprintf("Object index %d is out of range.", request.ObjectIndex),
		})
		return
	}
	obj := original.Objects[request.ObjectIndex]
	if request.FieldIndex < 0 || request.FieldIndex >= len(obj.Fields) {
		preview.CanApply = false
		preview.Warnings = append(preview.Warnings, hvacWarningForObject(obj, "hvac_field_out_of_range",
			fmt.Sprintf("Field index %d is out of range for %s.", request.FieldIndex, objectLabel(obj))))
		return
	}
	editField, ok := hvacEditableFieldAt(original, obj, request.FieldIndex)
	if !ok {
		preview.CanApply = false
		preview.Warnings = append(preview.Warnings, hvacWarningForObject(obj, "unsafe_hvac_field",
			fmt.Sprintf("%s field %d is not in the safe HVAC edit set.", objectLabel(obj), request.FieldIndex+1)))
		return
	}
	nextValue := strings.TrimSpace(request.Value)
	currentValue := strings.TrimSpace(obj.Fields[request.FieldIndex].Value)
	if currentValue == nextValue {
		preview.Changes = append(preview.Changes, HVACApplyChange{
			Action:       "no_change",
			ObjectIndex:  obj.Index,
			ObjectType:   obj.Type,
			ObjectName:   objectName(obj),
			FieldIndex:   request.FieldIndex,
			FieldName:    editField.FieldName,
			EditKind:     editField.EditKind,
			Before:       currentValue,
			After:        nextValue,
			Message:      fmt.Sprintf("%s stays at %q.", editField.FieldName, currentValue),
			RequiresSave: false,
		})
		return
	}

	if warnings := validateHVACEditValue(original, obj, editField, nextValue); len(warnings) > 0 {
		for _, warning := range warnings {
			if warning.Severity == DiagnosticError {
				preview.CanApply = false
			}
			preview.Warnings = append(preview.Warnings, warning)
		}
		if !preview.CanApply {
			return
		}
	}

	preview.Changes = append(preview.Changes, HVACApplyChange{
		Action:       "update_field",
		ObjectIndex:  obj.Index,
		ObjectType:   obj.Type,
		ObjectName:   objectName(obj),
		FieldIndex:   request.FieldIndex,
		FieldName:    editField.FieldName,
		EditKind:     editField.EditKind,
		Before:       currentValue,
		After:        nextValue,
		Message:      fmt.Sprintf("Update %s on %s from %q to %q.", editField.FieldName, objectLabel(obj), currentValue, nextValue),
		RequiresSave: true,
	})
	if mutate {
		updated.Objects[request.ObjectIndex].Fields[request.FieldIndex].Value = nextValue
	}
}

func validateHVACEditValue(doc Document, obj Object, editField HVACEditField, value string) []HVACWarning {
	var warnings []HVACWarning
	if strings.TrimSpace(value) == "" {
		return append(warnings, HVACWarning{
			Severity:    DiagnosticWarning,
			Category:    "HVAC Apply",
			Code:        "blank_hvac_value",
			Message:     fmt.Sprintf("%s will be cleared.", editField.FieldName),
			ObjectIndex: obj.Index,
			ObjectType:  obj.Type,
			ObjectName:  objectName(obj),
			FieldIndex:  editField.FieldIndex,
			Field:       editField.FieldName,
			Value:       value,
		})
	}
	switch editField.ValueType {
	case "number":
		if editField.AllowAutosize && isFlexibleSizingValue(value) {
			return warnings
		}
		if _, ok := parseFloatField(value); !ok {
			warnings = append(warnings, HVACWarning{
				Severity:    DiagnosticError,
				Category:    "HVAC Apply",
				Code:        "invalid_hvac_number",
				Message:     fmt.Sprintf("%s expects a numeric value or Autosize.", editField.FieldName),
				ObjectIndex: obj.Index,
				ObjectType:  obj.Type,
				ObjectName:  objectName(obj),
				FieldIndex:  editField.FieldIndex,
				Field:       editField.FieldName,
				Value:       value,
			})
		}
	case "integer":
		if parsed, ok := parseIntField(value); !ok || parsed < 1 {
			warnings = append(warnings, HVACWarning{
				Severity:    DiagnosticError,
				Category:    "HVAC Apply",
				Code:        "invalid_hvac_integer",
				Message:     fmt.Sprintf("%s expects a positive integer sequence.", editField.FieldName),
				ObjectIndex: obj.Index,
				ObjectType:  obj.Type,
				ObjectName:  objectName(obj),
				FieldIndex:  editField.FieldIndex,
				Field:       editField.FieldName,
				Value:       value,
			})
		}
	case "reference":
		if strings.Contains(editField.EditKind, "schedule") && !hvacScheduleExists(doc, value) {
			warnings = append(warnings, HVACWarning{
				Severity:    DiagnosticWarning,
				Category:    "HVAC Apply",
				Code:        "missing_hvac_schedule",
				Message:     fmt.Sprintf("Schedule %q was not found; EnergyPlus may reject this reference.", value),
				ObjectIndex: obj.Index,
				ObjectType:  obj.Type,
				ObjectName:  objectName(obj),
				FieldIndex:  editField.FieldIndex,
				Field:       editField.FieldName,
				Value:       value,
			})
		}
	}
	return warnings
}

func hvacScheduleExists(doc Document, value string) bool {
	for _, obj := range doc.Objects {
		if isScheduleType(obj.Type) && strings.EqualFold(objectName(obj), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func parseIntField(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
