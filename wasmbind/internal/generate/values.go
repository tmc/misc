package generate

import "github.com/tmc/misc/wasmbind/internal/schema"

func tsInputType(t *schema.Type) string {
	switch t.Kind {
	case "record":
		if t.Input {
			return t.Name + "Input"
		}
		return t.Name
	case "pointer":
		return tsInputType(t.Elem) + " | null"
	case "slice", "array":
		return "Array<" + tsInputType(t.Elem) + ">"
	}
	return tsType(t)
}
