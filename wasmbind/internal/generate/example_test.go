package generate_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

func ExampleFiles() {
	path, _ := filepath.Abs("../schema/testdata/hello")
	api, err := schema.Load(context.Background(), path, "")
	if err != nil {
		panic(err)
	}
	files, err := generate.Files(api)
	if err != nil {
		panic(err)
	}
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Println(names)
	// Output: [bindings.d.ts bindings.js bindings.json main.go]
}

func ExampleWrite() {
	root := filepath.Join(os.Getenv("HOME"), "tmp", "wasmbind", "examples")
	if err := os.MkdirAll(root, 0755); err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp(root, "run-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for name, text := range map[string]string{"go.mod": "module example.com/hello\n\ngo 1.27.0\n", "hello.go": "package hello\n//wasmbind:export echo\nfunc Echo(s string)string{return s}\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0644); err != nil {
			panic(err)
		}
	}
	api, err := schema.Load(context.Background(), dir, "")
	if err != nil {
		panic(err)
	}
	path, err := generate.Write(context.Background(), api, filepath.Join(dir, "cmd", "bind"), filepath.Join(dir, "web"), "", false)
	if err != nil {
		panic(err)
	}
	fmt.Println(filepath.Base(path))
	// Output: bindings.json
}
