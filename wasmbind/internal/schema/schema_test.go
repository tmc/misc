package schema

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestValidation(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"map", "//wasmbind:export f\nfunc F(v map[string]int) {}", "unsupported type"},
		{"reserved", "//wasmbind:export then\nfunc F() {}", "reserved"},
		{"duplicate", "//wasmbind:export f\nfunc F() {}\n//wasmbind:export f\nfunc G() {}", "duplicate JavaScript name"},
		{"private", "//wasmbind:export f\nfunc f() {}", "public"},
		{"variadic", "//wasmbind:export f\nfunc F(v ...int) {}", "variadic"},
		{"generic", "//wasmbind:export f\nfunc F[T any](v T) T {return v}", "unspecialized"},
		{"constraint", "//wasmbind:export f[string]\nfunc F[T ~int](v T) T {return v}", "does not satisfy"},
		{"recursive", "//wasmbind:record\ntype R struct {Next *R}", "recursive record"},
		{"embedded", "//wasmbind:record\ntype R struct {int}", "private and embedded"},
		{"private field", "//wasmbind:record\ntype R struct {n int}", "private and embedded"},
		{"resource close", "//wasmbind:resource R\ntype R struct {}", "close method"},
		{"value receiver", "//wasmbind:resource R\ntype R struct {}\n//wasmbind:export f\nfunc (R) F() {}", "pointer receiver"},
		{"results", "//wasmbind:export f\nfunc F()(int,string) {return 0,\"\"}", "only no result"},
		{"channel", "//wasmbind:export f\nfunc F(v chan int) {}", "unsupported type"},
		{"directive", "//wasmbind:wat\nfunc F() {}", "unknown directive"},
		{"contradictory", "//wasmbind:record\n//wasmbind:resource R\ntype R struct{}", "contradictory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fixture(t, "package fixture\n"+tt.source+"\n")
			_, err := Load(context.Background(), path, "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}
func TestAliasesGenericsAndStableSchema(t *testing.T) {
	path := fixture(t, `package fixture
//wasmbind:record
type R struct { A Alias; Skip chan int `+"`wasmbind:\"-\"`"+` }
type Alias = int64
//wasmbind:export f[int]
//wasmbind:export s[string]
func F[T ~int | ~string](v T) T {return v}
//wasmbind:export r
func Record(v *R) *R {return v}
`)
	a, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint != b.Fingerprint || len(a.Calls) != 3 {
		t.Fatalf("unstable schema or missing calls: %+v", a.Manifest)
	}
	if a.Declarations[0].Fields[0].Type.Kind != "int64" {
		t.Fatal("alias representation")
	}
	if strings.Contains(a.Calls[0].Doc, "wasmbind:") {
		t.Fatal("directive leaked into docs")
	}
}

func TestBuildTags(t *testing.T) {
	path := fixture(t, "package fixture\n//wasmbind:export echo\nfunc Echo(){}\n")
	source := "//go:build selected\n\npackage fixture\n//wasmbind:export tagged\nfunc Tagged(){}\n"
	if err := os.WriteFile(filepath.Join(path, "selected.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := Load(context.Background(), path, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(context.Background(), path, "selected")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Calls) != 1 || len(b.Calls) != 2 || a.Fingerprint == b.Fingerprint {
		t.Fatal("build tags were ignored")
	}
}

func TestJavaScriptIdentifiers(t *testing.T) {
	for _, name := range []string{"écho", "$hello", "_hello", "漢字", "a\u200c"} {
		if !validName(name) {
			t.Errorf("rejected valid name %q", name)
		}
	}
	for _, name := range []string{"", "0hello", "a-b", "\u200cabc", "😀", "then", "__proto__"} {
		if validName(name) {
			t.Errorf("accepted invalid name %q", name)
		}
	}
}
