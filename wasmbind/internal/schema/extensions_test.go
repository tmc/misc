package schema

import (
	"context"
	"strings"
	"testing"
)

func TestExtensionValidation(t *testing.T) {
	resource := "//wasmbind:resource R\ntype R struct{}\n//wasmbind:close\nfunc(*R)Close(){}\n"
	tests := []struct{ name, features, source, want string }{
		{"disabled", "", resource + "//wasmbind:read\nfunc(*R)Read(context.Context,[]byte)(int,error){return 0,nil}", "requires -features"},
		{"missing hook", "streams", resource + "//wasmbind:read\nfunc(*R)Read(context.Context,[]byte)(int,error){return 0,nil}", "cancelread"},
		{"write finish", "streams", resource + "//wasmbind:write\nfunc(*R)Write(context.Context,[]byte)(int,error){return 0,nil}\n//wasmbind:cancelwrite\nfunc(*R)CancelWrite()error{return nil}", "finish"},
		{"iterator hook", "events", resource + "//wasmbind:next\nfunc(*R)Next(context.Context)(int,bool,error){return 0,false,nil}", "interrupt"},
		{"bad read", "streams", resource + "//wasmbind:read\nfunc(*R)Read([]byte)(int,error){return 0,nil}", "requires (context"},
		{"bad imports", "imports", "//wasmbind:imports H\ntype H interface{ F(string)error }", "context first"},
		{"imports init", "imports", "//wasmbind:imports H\ntype H interface{ F(context.Context)error }", "init"},
		{"bad error", "errors", "//wasmbind:error Broken\ntype Broken struct{}", "implement error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, "package fixture\nimport \"context\"\nvar _ context.Context\n"+tt.source)
			_, err := LoadOptions(context.Background(), path, "", tt.features, "syscalljs")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
	for _, features := range []string{"unknown", "streams,streams"} {
		if _, err := LoadOptions(context.Background(), ".", "", features, "syscalljs"); err == nil {
			t.Fatalf("accepted %q", features)
		}
	}
	if _, err := LoadOptions(context.Background(), ".", "", "imports", "abi"); err == nil {
		t.Fatal("accepted unsupported ABI imports")
	}
}
