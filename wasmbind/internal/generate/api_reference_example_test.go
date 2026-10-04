package generate_test

import (
	"fmt"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/generate"
	"github.com/tmc/misc/wasmbind/internal/schema"
)

func ExampleAPIReference() {
	api := &schema.API{Manifest: schema.Manifest{
		Package: "example.com/hello",
		Calls:   []*schema.Call{{Name: "echo", Result: &schema.Type{Kind: "string"}, Params: []schema.Param{{Name: "text", Type: &schema.Type{Kind: "string"}}}}},
	}}
	reference := generate.APIReference(api)
	fmt.Println(strings.Contains(string(reference), "echo(text: string): Promise<string>;"))
	// Output: true
}
