package generate

import (
	"strings"
	"testing"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func TestDeclarationQuality(t *testing.T) {
	a := &schema.API{Manifest: schema.Manifest{
		Package:  "example.com/api",
		Features: []string{"imports", "streams"},
		Declarations: []*schema.Decl{
			{Kind: "scalar", Name: "PeerID", GoName: "PeerIdentifier", Value: &schema.Type{Kind: "string"}, Doc: "PeerIdentifier identifies a peer.\n\nIt is an opaque string."},
			{Kind: "enum", Name: "Kind", Validate: "enum", Value: &schema.Type{Kind: "uint64"}, Constants: []schema.Constant{{Name: "Connected", GoName: "KindConnected", Value: "1", Doc: "KindConnected identifies a connection."}}},
			{Kind: "record", Name: "Event", Fields: []schema.Field{{Name: "peerID", GoName: "PeerID", Type: &schema.Type{Kind: "string", Name: "PeerID"}, Doc: "PeerID identifies the sender."}}},
			{Kind: "resource", Name: "Endpoint"},
			{Kind: "imports", Name: "Host", Optional: true, Doc: "Host supplies callbacks.", ImportMethods: []*schema.Call{{Name: "fetch", GoName: "Fetch", Doc: "Fetch obtains a value.", Params: []schema.Param{{Name: "url", Type: &schema.Type{Kind: "string"}}}, Result: &schema.Type{Kind: "bytes"}}}},
		},
		Calls: []*schema.Call{{Name: "bind", GoName: "Bind", Doc: "Bind creates an Endpoint.", NonNull: true, Result: &schema.Type{Kind: "resource", Name: "Endpoint"}}},
	}}
	got := declarations(a)
	for _, want := range []string{
		"export type PeerID = string;",
		"readonly Connected: 1n;",
		"export type Kind = (typeof Kind)[keyof typeof Kind];",
		"  /** peerID identifies the sender. */\n  peerID: PeerID;",
		"  imports?: Imports;",
		"  Host?: {",
		"    /** fetch obtains a value. */\n    fetch(url: string, options: { readonly signal: AbortSignal }): Uint8Array | Promise<Uint8Array>;",
		"  /** bind creates an Endpoint. */\n  bind(): Promise<Endpoint>;",
		"/**\n * PeerID identifies a peer.\n *\n * It is an opaque string.\n */",
		"options?: LoadOptions",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("declarations missing %q:\n%s", want, got)
		}
	}
	if count := strings.Count(got, "export interface LoadOptions"); count != 1 {
		t.Errorf("LoadOptions declarations = %d, want 1", count)
	}
	ref := string(APIReference(a))
	for _, want := range []string{"<!-- " + schema.Marker + " -->", "Go package: `example.com/api`", "### `bind`", "bind(): Promise<Endpoint>;", "## Host imports", "export type PeerID = string;", "peerID identifies the sender"} {
		if !strings.Contains(ref, want) {
			t.Errorf("reference missing %q:\n%s", want, ref)
		}
	}
	a.Declarations[4].Optional = false
	got = declarations(a)
	if !strings.Contains(got, "  imports: Imports;") || strings.Contains(got, "options?: LoadOptions") || strings.Contains(got, "  Host?: {") {
		t.Errorf("required imports are optional:\n%s", got)
	}
}

func TestCallParameterNames(t *testing.T) {
	for _, imported := range []bool{false, true} {
		c := &schema.Call{Name: "call", Context: true, Params: []schema.Param{
			{Name: "await", Type: &schema.Type{Kind: "string"}},
			{Name: "arg0", Type: &schema.Type{Kind: "string"}},
			{Name: "options", Type: &schema.Type{Kind: "string"}},
			{Name: "optionsArg", Type: &schema.Type{Kind: "string"}},
		}}
		got := callSignature(c, imported)
		want := "call(arg0_2: string, arg0: string, optionsArg_2: string, optionsArg: string, options"
		if !strings.HasPrefix(got, want) {
			t.Errorf("imported=%v signature=%q, want prefix %q", imported, got, want)
		}
	}
}

func TestTypeNullability(t *testing.T) {
	cases := []struct {
		name   string
		typeOf *schema.Type
		want   string
	}{
		{"named number", &schema.Type{Kind: "uint8", Name: "Kind"}, "Kind"},
		{"nullable record", &schema.Type{Kind: "pointer", Elem: &schema.Type{Kind: "record", Name: "Data"}}, "Data | null"},
		{"nullable elements", &schema.Type{Kind: "slice", Elem: &schema.Type{Kind: "resource", Name: "Item"}}, "Array<Item | null>"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tsType(tt.typeOf); got != tt.want {
				t.Fatalf("tsType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocDelimiter(t *testing.T) {
	a := &schema.API{Manifest: schema.Manifest{Declarations: []*schema.Decl{{Kind: "scalar", Name: "Text", Value: &schema.Type{Kind: "string"}, Doc: "Text includes */ in documentation."}}}}
	if got := declarations(a); !strings.Contains(got, "/** Text includes * / in documentation. */") {
		t.Fatalf("unsafe doc comment:\n%s", got)
	}
}

func TestOpenEnumDeclaration(t *testing.T) {
	a := &schema.API{Manifest: schema.Manifest{Declarations: []*schema.Decl{{Kind: "enum", Name: "Kind", Value: &schema.Type{Kind: "uint8"}, Constants: []schema.Constant{{Name: "Known", Value: "1"}}}}}}
	got := declarations(a)
	if !strings.Contains(got, "readonly Known: 1;") || !strings.Contains(got, "export type Kind = number;") {
		t.Fatalf("unvalidated enum must accept unknown values:\n%s", got)
	}
}

func TestHiddenMethodDoc(t *testing.T) {
	a := &schema.API{Manifest: schema.Manifest{
		Declarations: []*schema.Decl{{Kind: "resource", Name: "Stream", Doc: "Stream uses Read and Write internally."}},
		Calls:        []*schema.Call{{Op: "read", GoName: "Read", Name: "$read", Receiver: "Stream"}, {Op: "write", GoName: "Write", Name: "$write", Receiver: "Stream"}},
	}}
	got := declarations(a)
	if !strings.Contains(got, "Stream uses Read and Write internally.") || strings.Contains(got, "$read") || strings.Contains(got, "$write") {
		t.Fatalf("hidden protocol names appeared in public docs:\n%s", got)
	}
}
