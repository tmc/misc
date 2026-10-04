package schema

import (
	"context"
	"strings"
	"testing"
)

func TestOwnership(t *testing.T) {
	const types = `package fixture
//wasmbind:resource Parent
type Parent struct{N int}
//wasmbind:close
func(*Parent)Close(){}
//wasmbind:resource Child
type Child struct{N int}
//wasmbind:close
func(*Child)Close(){}
`
	for _, backend := range []string{"syscalljs", "abi"} {
		t.Run(backend, func(t *testing.T) {
			path := fixture(t, types+`//wasmbind:export children
//wasmbind:owned
func(*Parent)Children()[]*Child{return nil}
//wasmbind:export child
//wasmbind:owned
//wasmbind:nonnull
func(*Parent)Child()*Child{return &Child{}}
`)
			a, err := LoadOptions(context.Background(), path, "", "", backend)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range a.Calls {
				if !c.Owned {
					t.Fatalf("missing ownership: %+v", c)
				}
			}
		})
	}
	for _, tt := range []struct{ name, source, want string }{
		{"function", "//wasmbind:export p\n//wasmbind:owned\nfunc P()*Parent{return nil}", "resource receiver"},
		{"scalar", "//wasmbind:export n\n//wasmbind:owned\nfunc(*Parent)NValue()int{return 0}", "resource result"},
		{"argument", "//wasmbind:export child\n//wasmbind:owned parent\nfunc(*Parent)Child()*Child{return nil}", "takes no argument"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(context.Background(), fixture(t, types+tt.source), "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}
