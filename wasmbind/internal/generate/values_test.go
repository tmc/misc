package generate

import (
	"github.com/tmc/misc/wasmbind/internal/schema"
	"strings"
	"testing"
)

func TestValueDeclarations(t *testing.T) {
	text := &schema.Type{Kind: "string"}
	a := &schema.API{Manifest: schema.Manifest{Declarations: []*schema.Decl{
		{Kind: "scalar", Name: "ID", Brand: true, Value: text},
		{Kind: "record", Name: "Child", Input: true, Fields: []schema.Field{{Name: "count", Optional: true, Type: &schema.Type{Kind: "int"}}}},
		{Kind: "record", Name: "Options", Input: true, Fields: []schema.Field{{Name: "child", Optional: true, Type: &schema.Type{Kind: "record", Name: "Child", Input: true}}}},
	}, Calls: []*schema.Call{{Name: "echo", Params: []schema.Param{{Name: "options", Type: &schema.Type{Kind: "record", Name: "Options", Input: true}}}, Result: &schema.Type{Kind: "record", Name: "Options", Input: true}}}}}
	got := declarations(a)
	for _, want := range []string{"declare const IDBrand: unique symbol", "string & { readonly [IDBrand]: true }", "export interface ChildInput", "count?: number", "child?: ChildInput", "options: OptionsInput", "Promise<Options>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "child?: Child;") {
		t.Fatal("optional input leaked into output")
	}
}
