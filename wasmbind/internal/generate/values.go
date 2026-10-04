package generate

import "github.com/tmc/misc/wasmbind/internal/schema"

func tsInputType(t *schema.Type) string {
	switch t.Kind {
	case "record":
		return t.Name + "Input"
	case "pointer":
		return tsInputType(t.Elem) + " | null"
	case "slice", "array":
		return "Array<" + tsInputType(t.Elem) + ">"
	}
	return tsType(t)
}
