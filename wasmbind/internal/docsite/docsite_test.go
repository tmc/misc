package docsite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	m := schema.Manifest{Generated: schema.Marker, SchemaVersion: 1, Package: "example.com/<unsafe>", Declarations: []*schema.Decl{{Kind: "record", Name: "Value", Doc: "<script>alert(1)</script>"}}, Calls: []*schema.Call{{Name: "create", Namespace: "session"}}}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "bindings.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "site")
	if err := Write(dir, out); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(page)
	if strings.Contains(text, "<script>") || !strings.Contains(text, "&lt;script&gt;") || !strings.Contains(text, "session.create") {
		t.Fatal(text)
	}
	links := regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(text, -1)
	if len(links) < 4 {
		t.Fatal("missing navigation")
	}
	for _, link := range links {
		if !strings.Contains(text, `id="`+link[1]+`"`) {
			t.Fatalf("broken anchor %s", link[1])
		}
	}
}
func TestCheckExamples(t *testing.T) {
	dir := t.TempDir()
	tsc := filepath.Join(dir, "tsc")
	script := "#!/bin/sh\n[ \"$1\" = --strict ] || exit 1\n[ \"$2\" = --noEmit ] || exit 1\nexit 0\n"
	if err := os.WriteFile(tsc, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := CheckExamples(context.Background(), tsc, "ES2022,DOM", []string{"consumer.ts"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckExamples(context.Background(), "", "", nil); err == nil {
		t.Fatal("missing compiler accepted")
	}
	os.WriteFile(tsc, []byte("#!/bin/sh\necho consumer.ts: invalid types\nexit 2\n"), 0755)
	if err := CheckExamples(context.Background(), tsc, "ES2022,DOM", []string{"consumer.ts"}); err == nil || !strings.Contains(err.Error(), "invalid types") {
		t.Fatal(err)
	}
}

func TestCheckExamplesIgnoresCallerConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("tsconfig.json", []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("consumer.ts", []byte("const value: number = 1;"), 0644); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "tmp")
	if err := os.Mkdir(tmp, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	t.Setenv("WASMBIND_TEST_EXAMPLE", filepath.Join(dir, "consumer.ts"))
	compiler := filepath.Join(dir, "wasmbind-test-tsc")
	script := `#!/bin/sh
[ ! -f tsconfig.json ] || exit 2
for argument do example_path="$argument"; done
[ "$example_path" = "$WASMBIND_TEST_EXAMPLE" ] || exit 3
[ -f "$example_path" ] || exit 4
`
	if err := os.WriteFile(compiler, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if err := CheckExamples(context.Background(), "./wasmbind-test-tsc", "ES2022,DOM", []string{"consumer.ts"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := CheckExamples(context.Background(), "wasmbind-test-tsc", "ES2022,DOM", []string{"consumer.ts"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("compiler temporary directory retained")
	}
}
