package main

import (
	"context"
	"os"

	"github.com/Gonie-Gonie/idf-analyzer/internal/idf"
)

type App struct {
	ctx context.Context
}

type TextEditResult struct {
	Text   string      `json:"text"`
	Report *idf.Report `json:"report"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) AnalyzeIDFText(text string) (*idf.Report, error) {
	doc, err := idf.Parse(text)
	if err != nil {
		return nil, err
	}
	report := idf.Analyze(doc)
	return &report, nil
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
	doc, err := idf.Parse(text)
	if err != nil {
		return nil, err
	}
	updated, err := idf.UpdateField(doc, objectIndex, fieldIndex, value)
	if err != nil {
		return nil, err
	}
	resultText := updated.String()
	report := idf.Analyze(updated)
	return &TextEditResult{Text: resultText, Report: &report}, nil
}

func (a *App) RemoveUnusedObjectsText(text string) (*TextEditResult, error) {
	doc, err := idf.Parse(text)
	if err != nil {
		return nil, err
	}
	updated, _ := idf.RemoveUnusedObjects(doc)
	resultText := updated.String()
	report := idf.Analyze(updated)
	return &TextEditResult{Text: resultText, Report: &report}, nil
}
