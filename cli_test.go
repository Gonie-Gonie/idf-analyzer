package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliFixtureIDF = `
Version,
  24.1;

Building,
  CLI Building;

Zone,
  Office;

Zone,
  Office;
`

func TestCLISummaryWritesText(t *testing.T) {
	input := writeCLITestInput(t, cliFixtureIDF)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"summary", "-format", "text", input}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI summary exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Building name: CLI Building") {
		t.Fatalf("summary output missing building name:\n%s", stdout.String())
	}
}

func TestCLIConvertYAML(t *testing.T) {
	input := writeCLITestInput(t, cliFixtureIDF)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"convert", "-to", "yaml", input}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI convert yaml exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "semantic_energyplus_model:") || !strings.Contains(stdout.String(), "duplicate_groups:") {
		t.Fatalf("semantic YAML output missing expected content:\n%s", stdout.String())
	}
}

func TestCLICleanSemanticDuplicates(t *testing.T) {
	input := writeCLITestInput(t, cliFixtureIDF)
	output := filepath.Join(t.TempDir(), "cleaned.idf")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"clean", "-rules", "none", "-semantic-duplicates", "-o", output, input}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI clean exit = %d, stderr = %s", code, stderr.String())
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read cleaned output: %v", err)
	}
	if !strings.Contains(string(content), "Office 2") {
		t.Fatalf("cleaned output did not rename duplicate zone:\n%s", string(content))
	}
}

func TestCLIConvertTableXLSX(t *testing.T) {
	input := writeCLITestInput(t, cliFixtureIDF)
	output := filepath.Join(t.TempDir(), "tables.xlsx")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"convert", "-to", "table", "-o", output, input}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCLI convert table exit = %d, stderr = %s", code, stderr.String())
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("open xlsx zip: %v", err)
	}
	defer archive.Close()
	foundSheet := false
	foundStyles := false
	for _, file := range archive.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			foundSheet = true
			text := readZipFileText(t, file)
			if !strings.Contains(text, "[Zone]") || !strings.Contains(text, "object_index") {
				t.Fatalf("sheet XML missing table markers/header:\n%s", text)
			}
		}
		if file.Name == "xl/styles.xml" {
			foundStyles = true
			text := readZipFileText(t, file)
			if !strings.Contains(text, `<b/>`) || !strings.Contains(text, `patternType="solid"`) {
				t.Fatalf("styles XML missing bold/fill styling:\n%s", text)
			}
		}
	}
	if !foundSheet || !foundStyles {
		t.Fatalf("xlsx entries found sheet=%t styles=%t", foundSheet, foundStyles)
	}
}

func writeCLITestInput(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.idf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func readZipFileText(t *testing.T, file *zip.File) string {
	t.Helper()
	reader, err := file.Open()
	if err != nil {
		t.Fatalf("open zip entry %s: %v", file.Name, err)
	}
	defer reader.Close()
	var b bytes.Buffer
	if _, err := b.ReadFrom(reader); err != nil {
		t.Fatalf("read zip entry %s: %v", file.Name, err)
	}
	return b.String()
}
