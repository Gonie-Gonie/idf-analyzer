package main

import (
	"context"
	"os"

	"github.com/Gonie-Gonie/idf-analyzer/internal/epinput"
	"github.com/Gonie-Gonie/idf-analyzer/internal/idf"
)

type App struct {
	ctx context.Context
}

type TextEditResult struct {
	Text     string      `json:"text"`
	Format   string      `json:"format"`
	Version  string      `json:"version,omitempty"`
	Report   *idf.Report `json:"report"`
	Warnings []string    `json:"warnings,omitempty"`
}

type InputAnalysisResult struct {
	Text    string         `json:"text,omitempty"`
	Format  string         `json:"format"`
	Version string         `json:"version,omitempty"`
	Model   *epinput.Model `json:"model"`
	EPJSON  string         `json:"epjson,omitempty"`
	Report  *idf.Report    `json:"report"`
}

type ConversionResult struct {
	Text     string   `json:"text"`
	Format   string   `json:"format"`
	Version  string   `json:"version,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ModelPatchResult = InputAnalysisResult

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) AnalyzeIDFText(text string) (*idf.Report, error) {
	result, err := a.AnalyzeInputText(text)
	if err != nil {
		return nil, err
	}
	return result.Report, nil
}

func (a *App) AnalyzeInputText(text string) (*InputAnalysisResult, error) {
	model, err := epinput.Parse("", []byte(text))
	if err != nil {
		return nil, err
	}
	doc := epinput.ToIDFDocument(model)
	report := idf.Analyze(doc)
	epjsonText, err := epinput.Write(model, epinput.FormatEPJSON)
	if err != nil {
		return nil, err
	}

	return &InputAnalysisResult{
		Text:    text,
		Format:  string(model.Format),
		Version: model.Version.Raw,
		Model:   model,
		EPJSON:  epjsonText,
		Report:  &report,
	}, nil
}

func (a *App) PatchModelValueText(text string, objectIndex int, fieldIndex int, jsonPath []string, rawValue string) (*ModelPatchResult, error) {
	model, err := epinput.Parse("", []byte(text))
	if err != nil {
		return nil, err
	}
	if err := epinput.PatchFieldValue(model, objectIndex, fieldIndex, jsonPath, rawValue); err != nil {
		return nil, err
	}

	resultText, err := epinput.Write(model, model.Format)
	if err != nil {
		return nil, err
	}
	doc := epinput.ToIDFDocument(model)
	report := idf.Analyze(doc)
	epjsonText, err := epinput.Write(model, epinput.FormatEPJSON)
	if err != nil {
		return nil, err
	}

	return &ModelPatchResult{
		Text:    resultText,
		Format:  string(model.Format),
		Version: model.Version.Raw,
		Model:   model,
		EPJSON:  epjsonText,
		Report:  &report,
	}, nil
}

func (a *App) OpenIDF(path string) (*idf.Report, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return a.AnalyzeIDFText(string(content))
}

func (a *App) SaveIDF(path string, text string) error {
	return os.WriteFile(path, []byte(text), 0o644)
}

func (a *App) UpdateFieldText(text string, objectIndex int, fieldIndex int, value string) (*TextEditResult, error) {
	model, err := epinput.Parse("", []byte(text))
	if err != nil {
		return nil, err
	}
	doc := epinput.ToIDFDocument(model)
	updated, err := idf.UpdateField(doc, objectIndex, fieldIndex, value)
	if err != nil {
		return nil, err
	}
	resultText := writeDocumentInOriginalFormat(updated, model)
	report := idf.Analyze(updated)
	return &TextEditResult{
		Text:    resultText,
		Format:  string(model.Format),
		Version: model.Version.Raw,
		Report:  &report,
	}, nil
}

func (a *App) RemoveUnusedObjectsText(text string) (*TextEditResult, error) {
	model, err := epinput.Parse("", []byte(text))
	if err != nil {
		return nil, err
	}
	doc := epinput.ToIDFDocument(model)
	updated, _ := idf.RemoveUnusedObjects(doc)
	resultText := writeDocumentInOriginalFormat(updated, model)
	report := idf.Analyze(updated)
	return &TextEditResult{
		Text:    resultText,
		Format:  string(model.Format),
		Version: model.Version.Raw,
		Report:  &report,
	}, nil
}

func (a *App) ConvertInputText(text string, targetFormat string) (*ConversionResult, error) {
	model, err := epinput.Parse("", []byte(text))
	if err != nil {
		return nil, err
	}

	target := epinput.Format(targetFormat)
	target = epinput.NormalizeFormat(target)
	output, err := epinput.Write(model, target)
	if err != nil {
		return nil, err
	}

	return &ConversionResult{
		Text:    output,
		Format:  string(target),
		Version: model.Version.Raw,
	}, nil
}

func writeDocumentInOriginalFormat(doc idf.Document, original *epinput.Model) string {
	if original != nil && original.Format == epinput.FormatEPJSON {
		model := epinput.FromIDFDocument(doc, epinput.FormatEPJSON)
		output, err := epinput.Write(model, epinput.FormatEPJSON)
		if err == nil {
			return output
		}
	}
	return doc.String()
}
