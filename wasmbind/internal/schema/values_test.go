package schema

import (
	"context"
	"strings"
	"testing"
)

func TestValues(t *testing.T) {
	path := fixture(t, `package fixture
//wasmbind:scalar ID
//wasmbind:brand
//wasmbind:validate ValidateID
type ID string
func ValidateID(ID)error{return nil}
//wasmbind:record
type Options struct {Count int `+"`wasmbind:\"count,optional\" wasmbinddefault:\"7\"`"+`; ID ID}
//wasmbind:export echo
func Echo(options Options)Options{return options}
`)
	a, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	var helper bool
	for _, c := range a.Calls {
		if c.Name == "parseID" {
			helper = c.Scalar == "ID" && c.Context && c.Result.Name == "ID" && c.Params[0].Type.Name == ""
		}
	}
	if !helper {
		t.Fatal("missing validated scalar constructor")
	}
	for _, d := range a.Declarations {
		if d.Name == "Options" && (!d.Fields[0].Optional || d.Fields[0].Default != "7") {
			t.Fatalf("field: %+v", d.Fields[0])
		}
	}
}

func TestValueDiagnostics(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"brand without validator", "//wasmbind:scalar S\n//wasmbind:brand\ntype S string", "validated scalar"},
		{"unknown field option", "//wasmbind:record\ntype R struct {N int `wasmbind:\"n,other\"`}", "field option"},
		{"default required", "//wasmbind:record\ntype R struct {N int `wasmbinddefault:\"7\"`}", "requires optional"},
		{"default function", "func Default()int{return 7}\n//wasmbind:record\ntype R struct {N int `wasmbind:\"n,optional\" wasmbinddefault:\"Default()\"`}", "constant assignable"},
		{"default overflow", "//wasmbind:record\ntype R struct {N uint8 `wasmbind:\"n,optional\" wasmbinddefault:\"256\"`}", "out of range"},
		{"default unsafe integer", "//wasmbind:record\ntype R struct {N int `wasmbind:\"n,optional\" wasmbinddefault:\"9007199254740993\"`}", "exactly representable"},
		{"input collision", "//wasmbind:record\ntype R struct{}\n//wasmbind:record\ntype RInput struct{}", "duplicate JavaScript name"},
		{"helper collision", "//wasmbind:scalar ID\n//wasmbind:brand\n//wasmbind:validate V\ntype ID string\nfunc V(ID)error{return nil}\n//wasmbind:export parseID\nfunc Parse(string)ID{return \"\"}", "duplicate JavaScript name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, "package fixture\n"+tt.body)
			_, err := Load(context.Background(), path, "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}
