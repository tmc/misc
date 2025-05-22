# JSONSchema Generation with protoc-gen-anything

This example shows how to use `protoc-gen-anything` to generate JSONSchema from Protocol Buffer definitions using custom templates.

## Features

- Uses the templating capabilities of protoc-gen-anything
- Generates JSONSchema for each Protocol Buffer message
- Handles nested messages, enums, maps, and repeated fields
- Configurable options for schema generation

## Usage

Run the example:

```bash
make generate
```

This will generate JSONSchema files in the `gen` directory.

## Structure

- `jsonschemagen/`: Contains the JSONSchema generation library
  - `jsonschema.go`: The core schema generation logic
  - `template_funcs.go`: Helper functions for templates
- `templates/`: Contains the templates for schema generation
- `proto/`: Example proto files
- `main.go`: The protoc-gen-anything plugin entry point

## Customization

You can customize the schema generation by modifying the templates or adding your own options to the `main.go` file.

## Using as a Library

The `jsonschemagen` package can be used as a library in your own projects. Import it and use it with your custom templates:

```go
import (
    "text/template"
    
    "github.com/tmc/misc/protoc-gen-anything/examples/jsonschema/jsonschemagen"
)

func setupTemplate() *template.Template {
    funcMap := template.FuncMap{}
    funcMap = jsonschemagen.AddTemplateFuncs(funcMap, true, false)
    
    return template.New("schema").Funcs(funcMap)
}
```