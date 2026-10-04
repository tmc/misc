package schema

import (
	"context"
	"strings"
	"testing"
)

func TestPolicies(t *testing.T) {
	path := fixture(t, `// Package fixture tests JavaScript naming.
//wasmbind:naming camelCase
package fixture
import "context"
// Identifier is a validated domain value.
//wasmbind:scalar Identifier
//wasmbind:validate ValidateIdentifier
type Identifier string
func ValidateIdentifier(v Identifier)error{return nil}
//wasmbind:enum Kind
//wasmbind:validate
type Kind uint64
const (
 First Kind=0
 Alias Kind=0
 Last Kind=18446744073709551615
)
//wasmbind:record
type Record struct {
 // HTTPStatus is the status code.
 HTTPStatus int
 URL string
 Override string `+"`wasmbind:\"Exact\"`"+`
 Kind Kind
}
//wasmbind:export
//wasmbind:nonnull
func HTTPResult() *Record{return &Record{}}
//wasmbind:imports Host
//wasmbind:optional DefaultHost
type Host interface {
 // FetchURL receives a domain identifier.
 //wasmbind:export fetch
 FetchURL(ctx context.Context, identifier Identifier)([]byte,error)
}
type defaultHost struct{}
func (defaultHost) FetchURL(context.Context,Identifier)([]byte,error){return nil,nil}
func DefaultHost() Host{return defaultHost{}}
//wasmbind:init
func Initialize(ctx context.Context,h Host)error{return nil}
`)
	a, err := LoadOptions(context.Background(), path, "", "imports", "syscalljs")
	if err != nil {
		t.Fatal(err)
	}
	if a.Naming != "camelCase" || a.Calls[0].Name != "httpResult" || !a.Calls[0].NonNull {
		t.Fatalf("wrong call policy: %+v", a.Calls)
	}
	for _, d := range a.Declarations {
		switch d.Name {
		case "Record":
			if d.Fields[0].Name != "httpStatus" || d.Fields[1].Name != "url" || d.Fields[2].Name != "Exact" || d.Fields[0].Doc == "" || d.Fields[3].Type.Name != "Kind" {
				t.Fatalf("wrong fields: %+v", d.Fields)
			}
		case "Host":
			if !d.Optional || d.Default != "DefaultHost" || d.ImportMethods[0].Name != "fetch" || d.ImportMethods[0].Params[0].Name != "identifier" || d.ImportMethods[0].Doc == "" {
				t.Fatalf("wrong imports: %+v", d)
			}
		case "Kind":
			if len(d.Constants) != 3 || d.Constants[0].Value != d.Constants[1].Value {
				t.Fatalf("enum aliases: %+v", d)
			}
		}
	}
}

func TestPolicyDiagnostics(t *testing.T) {
	tests := []struct{ name, source, features, backend, want string }{
		{"nonnull scalar", "//wasmbind:export f\n//wasmbind:nonnull\nfunc F()int{return 0}", "", "syscalljs", "pointer or resource"},
		{"nonnull alone", "//wasmbind:nonnull\nfunc F(){}", "", "syscalljs", "requires export"},
		{"field collision", "//wasmbind:record\ntype R struct{URL string;Url string}", "", "syscalljs", "duplicate record field"},
		{"method collision", "//wasmbind:export\nfunc URL(){}\n//wasmbind:export\nfunc Url(){}", "", "syscalljs", "duplicate JavaScript name"},
		{"enum empty", "//wasmbind:enum E\ntype E int", "", "syscalljs", "typed constant"},
		{"enum floats", "//wasmbind:enum E\ntype E float64", "", "syscalljs", "string, bool, or integer"},
		{"enum invalid UTF-8", "//wasmbind:enum E\ntype E string\nconst Bad E=\"\\xff\"", "", "syscalljs", "valid UTF-8"},
		{"enum unsafe int", "//wasmbind:enum E\ntype E int\nconst Big E=9007199254740993", "", "syscalljs", "exactly representable"},
		{"scalar validator signature", "//wasmbind:scalar S\n//wasmbind:validate V\ntype S string\nfunc V(string)error{return nil}", "", "syscalljs", "func(T) error"},
		{"scalar ABI validator", "//wasmbind:scalar S\n//wasmbind:validate V\ntype S string\nfunc V(S)error{return nil}", "", "abi", "require syscalljs"},
		{"optional factory", "//wasmbind:imports H\n//wasmbind:optional Missing\ntype H interface{F(context.Context)error}", "imports", "syscalljs", "default factory"},
		{"optional factory signature", "//wasmbind:imports H\n//wasmbind:optional Default\ntype H interface{F(context.Context)error}\nfunc Default()error{return nil}", "imports", "syscalljs", "func() T"},
		{"bad method directive", "//wasmbind:imports H\ntype H interface{\n//wasmbind:nope\nF(context.Context)error\n}", "imports", "syscalljs", "unknown directive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, "//wasmbind:naming camelCase\npackage fixture\nimport \"context\"\nvar _ context.Context\n"+tt.source)
			_, err := LoadOptions(context.Background(), path, "", tt.features, tt.backend)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestUnusedImportsFeature(t *testing.T) {
	a, err := LoadOptions(context.Background(), fixture(t, "package fixture\n//wasmbind:export f\nfunc F(){}"), "", "imports", "syscalljs")
	if err != nil || a.Init != nil {
		t.Fatalf("unused imports feature: %v", err)
	}
}
