package codegen

import (
	"encoding/json"
	"strings"
	"testing"

	"generator/descriptor"
)

func library() map[string]descriptor.ClassDescriptor {
	return map[string]descriptor.ClassDescriptor{
		"thermometer": {
			Name: "thermometer",
			Registers: []descriptor.RegisterDescriptor{
				{Name: "temperature", Type: descriptor.TypeFloat, Multiplier: 1, Divider: 100, Metadata: map[string]string{"unit": "celsius"}},
				{Name: "humidity", Type: descriptor.TypeInt, Multiplier: 1, Divider: 1},
			},
		},
		"switch": {
			Name:      "switch",
			Registers: []descriptor.RegisterDescriptor{{Name: "relay", Type: descriptor.TypeBool}},
		},
	}
}

func garage() descriptor.Resolved {
	spec := descriptor.NodeSpec{
		Name:     "garage-controller",
		Channel:  10,
		Metadata: map[string]string{"hw_rev": "1.3"},
		Instances: []descriptor.ClassInstance{
			{Class: "thermometer", Name: "outdoor"},
			{Class: "switch", Name: "main"},
			{Class: "switch", Name: "aux"},
		},
	}
	res, err := descriptor.AllocateIDs(spec, library())
	if err != nil {
		panic(err)
	}
	return res
}

func TestGenerateNodeCode_CompilableAndComplete(t *testing.T) {
	res := garage()
	src, err := GenerateNodeCode(res, NodeCodeOptions{Package: "nodefw"})
	if err != nil {
		t.Fatalf("GenerateNodeCode: %v", err)
	}
	code := string(src)

	// format.Source already proves it parses & is gofmt-clean; spot-check content.
	if !strings.Contains(code, "package nodefw") {
		t.Error("missing package clause")
	}
	if !strings.Contains(code, "DO NOT EDIT") {
		t.Error("missing generated-code marker")
	}
	if !strings.Contains(code, "const DescriptorVersion uint32 = 0x") {
		t.Error("missing DescriptorVersion const")
	}
	if !strings.Contains(code, "var RegisterIDs = []uint16{") {
		t.Error("missing RegisterIDs slice")
	}
	// Expect a const per register, derived from qualified names.
	for _, want := range []string{"RegAuxRelay", "RegMainRelay", "RegOutdoorHumidity", "RegOutdoorTemperature"} {
		if !strings.Contains(code, want) {
			t.Errorf("missing const %s", want)
		}
	}
	// No class-specific wrappers per requirement.
	for _, forbidden := range []string{"interface", "Thermometer", "Switch"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("generated code must not contain class wrapper %q", forbidden)
		}
	}
}

func TestGenerateNodeCode_Deterministic(t *testing.T) {
	a, _ := GenerateNodeCode(garage(), NodeCodeOptions{})
	b, _ := GenerateNodeCode(garage(), NodeCodeOptions{})
	if string(a) != string(b) {
		t.Error("node code generation is not deterministic")
	}
}

func TestGenerateNodeCode_DefaultPackage(t *testing.T) {
	src, err := GenerateNodeCode(garage(), NodeCodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "package main") {
		t.Error("expected default package main")
	}
}

func TestGenerateDescriptorJSON_Shape(t *testing.T) {
	res := garage()
	data, err := GenerateDescriptorJSON(res)
	if err != nil {
		t.Fatalf("GenerateDescriptorJSON: %v", err)
	}

	var jd jsonDescriptor
	if err := json.Unmarshal(data, &jd); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if jd.Channel != 10 {
		t.Errorf("channel wrong: %d", jd.Channel)
	}
	if !strings.HasPrefix(jd.Version, "0x") {
		t.Errorf("version not hex string: %q", jd.Version)
	}
	if len(jd.Registers) != 4 {
		t.Fatalf("got %d registers, want 4", len(jd.Registers))
	}
	// Canonical order preserved.
	wantNames := []string{"aux.relay", "main.relay", "outdoor.humidity", "outdoor.temperature"}
	for i, r := range jd.Registers {
		if r.Name != wantNames[i] {
			t.Errorf("register %d = %q, want %q", i, r.Name, wantNames[i])
		}
		if r.ID == 0 {
			t.Errorf("register %q has reserved ID 0", r.Name)
		}
		if r.Metadata == nil {
			t.Errorf("register %q metadata is null, want {}", r.Name)
		}
	}
	// Scaling/metadata carried through from the class descriptor.
	temp := jd.Registers[3]
	if temp.Name != "outdoor.temperature" || temp.Type != "float" || temp.Divider != 100 {
		t.Errorf("temperature register wrong: %+v", temp)
	}
	if temp.Metadata["unit"] != "celsius" {
		t.Errorf("temperature unit metadata lost: %v", temp.Metadata)
	}
}

func TestGenerateDescriptorJSON_NilMetadataRendersEmpty(t *testing.T) {
	res := garage()
	res.Metadata = nil
	data, err := GenerateDescriptorJSON(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("nil maps must render as {} not null:\n%s", data)
	}
}

func TestExportIdent(t *testing.T) {
	cases := map[string]string{
		"outdoor.temperature": "OutdoorTemperature",
		"relay_a.state":       "RelayAState",
		"a.b-c":               "ABC",
		"123.bad":             "X123Bad",
	}
	for in, want := range cases {
		if got := exportIdent(in); got != want {
			t.Errorf("exportIdent(%q) = %q, want %q", in, got, want)
		}
	}
}
