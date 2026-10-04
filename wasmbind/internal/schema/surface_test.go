package schema

import (
	"context"
	"strings"
	"testing"
)

const unionFixture = `//wasmbind:naming camelCase
package fixture
//wasmbind:enum Kind
//wasmbind:validate
type Kind uint8
const (
 //wasmbind:member message
 KindMessage Kind=1
 //wasmbind:member lagged
 KindLagged Kind=2
)
//wasmbind:record
//wasmbind:union Kind
//wasmbind:variant KindMessage:Content
//wasmbind:variant KindLagged:Dropped
type Event struct {Kind Kind; Content []byte; Dropped uint64}
//wasmbind:export echo
//wasmbind:namespace events
func Echo(e Event)Event{return e}
`

func TestSurfacePolicies(t *testing.T) {
	path := fixture(t, unionFixture)
	api, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range api.Declarations {
		if d.Name == "Event" {
			if d.Discriminant != "kind" || len(d.Variants) != 2 || d.Variants[0].Constant != "Kind.message" {
				t.Fatalf("union = %#v", d)
			}
		}
	}
	if api.Calls[0].Namespace != "events" {
		t.Fatal(api.Calls[0])
	}
}
func TestSurfaceErrors(t *testing.T) {
	for _, tt := range []struct{ name, old, new, want string }{
		{"missing", "//wasmbind:variant KindLagged:Dropped", "// omitted variant", "missing variant"},
		{"field", "KindMessage:Content", "KindMessage:Unknown", "unknown union variant field"},
		{"duplicate", "KindLagged:Dropped", "KindMessage:Dropped", "duplicate union variant"},
		{"namespace", "namespace events", "namespace then", "valid name"},
		{"collision", "func Echo(e Event)Event{return e}", "func Echo(e Event)Event{return e}\n//wasmbind:export events\nfunc Events(){}", "collides"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(context.Background(), fixture(t, strings.Replace(unionFixture, tt.old, tt.new, 1)), "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want %s", err, tt.want)
			}
		})
	}
}
