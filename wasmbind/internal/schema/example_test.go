package schema_test

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func ExampleLoad() {
	path, _ := filepath.Abs("testdata/hello")
	api, err := schema.Load(context.Background(), path, "")
	if err != nil {
		panic(err)
	}
	fmt.Println(api.Package, len(api.Calls))
	// Output: example.com/hello 1
}
