package webidl2plan9

import (
	"os"
	"strings"
	"testing"
)

func loadPromptAPI(t *testing.T) *IDL {
	t.Helper()
	src, err := os.ReadFile("testdata/prompt-api.idl")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	idl, err := Parse(string(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return idl
}

func TestParsePromptAPI(t *testing.T) {
	idl := loadPromptAPI(t)

	if got, want := len(idl.Interfaces), 2; got != want {
		t.Errorf("interfaces = %d, want %d", got, want)
	}
	lm := findIface(idl, "LanguageModel")
	if lm == nil {
		t.Fatal("LanguageModel interface not parsed")
	}
	// create() is static and returns Promise<LanguageModel>
	create := findOp(lm, "create")
	if create == nil || !create.Static {
		t.Fatalf("create op missing or not static: %+v", create)
	}
	if !returnsInterface(create.Return, "LanguageModel") {
		t.Errorf("create return = %q, want Promise<LanguageModel>", create.Return)
	}
	// measureContextUsage returns Promise<double> — the surface the hand design misstated
	mcu := findOp(lm, "measureContextUsage")
	if mcu == nil || !strings.Contains(mcu.Return, "double") {
		t.Errorf("measureContextUsage return = %q, want Promise<double>", mustReturn(mcu))
	}
	// deprecated members are flagged
	iu := findAttr(lm, "inputUsage")
	if iu == nil || !iu.Deprecated {
		t.Errorf("inputUsage should be flagged deprecated: %+v", iu)
	}
	// the DestroyableModel mixin is recorded
	if mixins := idl.Includes["LanguageModel"]; len(mixins) == 0 || !strings.Contains(mixins[0], "Destroyable") {
		t.Errorf("LanguageModel includes = %v, want DestroyableModel", idl.Includes["LanguageModel"])
	}
}

func TestClassifyInstanceConnection(t *testing.T) {
	idl := loadPromptAPI(t)
	shape, why, primary := idl.Classify()
	if shape != "instance-connection" {
		t.Errorf("shape = %q, want instance-connection", shape)
	}
	if primary == nil || primary.Name != "LanguageModel" {
		t.Errorf("primary = %v, want LanguageModel", primary)
	}
	if !strings.Contains(why, "create") {
		t.Errorf("why = %q, want mention of create()", why)
	}
}

func TestRenderDeterministicSurfaces(t *testing.T) {
	idl := loadPromptAPI(t)
	out := Render(idl, "Chrome Prompt API", "/llm")

	// the surfaces the hand-written it8 design got wrong must be correct here
	checks := []struct{ desc, want string }{
		{"create maps to clone", "`LanguageModel.create()` | operation | `/llm/clone`"},
		{"measureContextUsage is numeric", "Promise<double>"},
		{"deprecated inputUsage flagged", "inputUsage"},
		{"no invented prepare verb", ""}, // checked below as a negative
		{"object fails closed", "`object` (in `LanguageModelTool.inputSchema`)"},
		{"shape stated", "Shape: **instance-connection**"},
	}
	for _, c := range checks {
		if c.want != "" && !strings.Contains(out, c.want) {
			t.Errorf("%s: output missing %q", c.desc, c.want)
		}
	}
	// negative: the generator must NOT invent service verbs absent from the IDL
	for _, invented := range []string{"prepare", "Ask the service to make the model ready"} {
		if strings.Contains(out, invented) {
			t.Errorf("output invented a surface not in the IDL: %q", invented)
		}
	}
}

func findIface(idl *IDL, name string) *Interface {
	for _, i := range idl.Interfaces {
		if i.Name == name {
			return i
		}
	}
	return nil
}

func findOp(i *Interface, name string) *Operation {
	for _, o := range i.Operations {
		if o.Name == name {
			return o
		}
	}
	return nil
}

func findAttr(i *Interface, name string) *Attribute {
	for _, a := range i.Attributes {
		if a.Name == name {
			return a
		}
	}
	return nil
}

func mustReturn(o *Operation) string {
	if o == nil {
		return "<nil>"
	}
	return o.Return
}
