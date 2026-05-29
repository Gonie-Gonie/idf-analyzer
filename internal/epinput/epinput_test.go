package epinput

import (
	"strings"
	"testing"
)

const idfV22 = `
Version,
  22.2;                    !- Version Identifier

Zone,
  Office;                  !- Name
`

const epjsonV22 = `{
  "Version": {
    "Version 1": {
      "version_identifier": "22.2",
      "idf_order": 1
    }
  },
  "Zone": {
    "Office": {
      "direction_of_relative_north": 0,
      "idf_order": 2
    }
  },
  "Fan:ConstantVolume": {
    "Supply Fan": {
      "air_inlet_node_name": "Air Inlet Node",
      "air_outlet_node_name": "Air Outlet Node",
      "idf_order": 3
    }
  }
}`

func TestDetectFormat(t *testing.T) {
	if got := DetectFormat("model.idf", []byte("{}")); got != FormatIDF {
		t.Fatalf("idf extension format = %s, want idf", got)
	}
	if got := DetectFormat("model.epJSON", nil); got != FormatEPJSON {
		t.Fatalf("epJSON extension format = %s, want epjson", got)
	}
	if got := DetectFormat("", []byte("  {")); got != FormatEPJSON {
		t.Fatalf("json content format = %s, want epjson", got)
	}
}

func TestParseIDFDetectsSupportedVersionAndWritesEPJSON(t *testing.T) {
	model, err := Parse("model.idf", []byte(idfV22))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if model.Format != FormatIDF {
		t.Fatalf("format = %s, want idf", model.Format)
	}
	if !model.Version.Supported || model.Version.Major != 22 {
		t.Fatalf("version = %#v, want supported 22.x", model.Version)
	}

	epjson, err := Write(model, FormatEPJSON)
	if err != nil {
		t.Fatalf("Write(epjson) error = %v", err)
	}
	for _, want := range []string{`"Version"`, `"version_identifier": "22.2"`, `"Zone"`, `"Office"`} {
		if !strings.Contains(epjson, want) {
			t.Fatalf("epJSON output missing %q:\n%s", want, epjson)
		}
	}
}

func TestParseEPJSONDetectsVersionAndWritesIDF(t *testing.T) {
	model, err := Parse("model.epJSON", []byte(epjsonV22))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if model.Format != FormatEPJSON {
		t.Fatalf("format = %s, want epjson", model.Format)
	}
	if !model.Version.Supported || model.Version.Raw != "22.2" {
		t.Fatalf("version = %#v, want supported 22.2", model.Version)
	}

	idfText, err := Write(model, FormatIDF)
	if err != nil {
		t.Fatalf("Write(idf) error = %v", err)
	}
	for _, want := range []string{"Version,", "22.2", "Zone,", "Office", "Fan:ConstantVolume", "Air Inlet Node"} {
		if !strings.Contains(idfText, want) {
			t.Fatalf("IDF output missing %q:\n%s", want, idfText)
		}
	}
}

func TestRejectsPre22VersionWhenKnown(t *testing.T) {
	_, err := Parse("old.idf", []byte("Version,\n  9.6; !- Version Identifier\n"))
	if err == nil {
		t.Fatalf("Parse() error = nil, want unsupported version error")
	}
	if !strings.Contains(err.Error(), "version 22 or newer") {
		t.Fatalf("error = %q, want supported range message", err)
	}
}

func TestPatchFieldValueUpdatesRootAndNestedValues(t *testing.T) {
	model, err := Parse("model.epJSON", []byte(`{
  "Version": {
    "Version 1": {
      "version_identifier": "22.2",
      "idf_order": 1
    }
  },
  "Zone": {
    "Office": {
      "direction_of_relative_north": 0,
      "metadata": {"tags": ["old"]},
      "idf_order": 2
    }
  }
}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := PatchFieldValue(model, 1, 0, nil, "15"); err != nil {
		t.Fatalf("PatchFieldValue(root) error = %v", err)
	}
	if err := PatchFieldValue(model, 1, 1, []string{"tags", "0"}, `"new"`); err != nil {
		t.Fatalf("PatchFieldValue(nested) error = %v", err)
	}

	epjson, err := Write(model, FormatEPJSON)
	if err != nil {
		t.Fatalf("Write(epjson) error = %v", err)
	}
	for _, want := range []string{`"direction_of_relative_north": 15`, `"metadata": {"tags": ["new"]}`} {
		if !strings.Contains(epjson, want) {
			t.Fatalf("patched epJSON missing %q:\n%s", want, epjson)
		}
	}
}
