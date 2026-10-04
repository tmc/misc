package generate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func (e *emitter) validate(b *bytes.Buffer, d *schema.Decl, value, bad string, deferred bool) {
	if d.Validate == "" {
		return
	}
	if d.Validate != "enum" {
		if deferred {
			fmt.Fprintf(b, "validate:=func()error{if err:=api.%s(%s);err!=nil{return fmt.Errorf(\"validate %s: %%w\",err)};return nil}\nif i!=nil && i.deferValidation {i.validations=append(i.validations,validate)} else if err:=validate();err!=nil{%s}\n", d.Validate, value, d.Name, bad)
		} else {
			fmt.Fprintf(b, "if err:=api.%s(%s);err!=nil{err=fmt.Errorf(\"validate %s: %%w\",err);%s}\n", d.Validate, value, d.Name, bad)
		}
		return
	}
	var cases []string
	seen := map[string]bool{}
	for _, c := range d.Constants {
		if !seen[c.Value] {
			cases = append(cases, "api."+c.GoName)
			seen[c.Value] = true
		}
	}
	fmt.Fprintf(b, "switch %s {case %s:default:err:=failure(\"argument\",\"invalid %s value\");%s}\n", value, strings.Join(cases, ","), d.Name, bad)
}

func enumExports(a *schema.API) string {
	var b bytes.Buffer
	for _, d := range a.Declarations {
		if d.Kind != "enum" {
			continue
		}
		fmt.Fprintf(&b, "\nexport const %s = Object.freeze({\n", d.Name)
		for _, c := range d.Constants {
			value := c.Value
			if d.Value.Kind == "int64" || d.Value.Kind == "uint64" {
				value += "n"
			}
			fmt.Fprintf(&b, "  %s: %s,\n", c.Name, value)
		}
		fmt.Fprintln(&b, "});")
	}
	return b.String()
}
