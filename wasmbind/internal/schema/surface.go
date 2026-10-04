package schema

import (
	"fmt"
	"strings"
)

func (l *loader) unions() error {
	for _, d := range l.api.Declarations {
		if d.unionSpec == "" && d.variantSpec == "" {
			continue
		}
		obj := l.api.Pkg.Types.Scope().Lookup(d.GoName)
		bad := func(s string) error { return l.diag(obj.Pos(), d.GoName, s) }
		var tag *Field
		for i := range d.Fields {
			if d.Fields[i].GoName == d.unionSpec {
				tag = &d.Fields[i]
			}
		}
		if tag != nil && tag.Optional {
			return bad("union discriminant cannot be optional")
		}
		if tag == nil || d.variantSpec == "" {
			return bad("union requires a discriminant Go field and variant declarations")
		}
		var enum *Decl
		for _, e := range l.api.Declarations {
			if e.Name == tag.Type.Name && e.Kind == "enum" && e.Validate == "enum" {
				enum = e
			}
		}
		if enum == nil {
			return bad("union discriminant requires a validated enum")
		}
		d.Discriminant = tag.Name
		seen := map[string]bool{}
		for _, spec := range strings.Split(d.variantSpec, "\n") {
			parts := strings.SplitN(spec, ":", 2)
			var c *Constant
			for i := range enum.Constants {
				if enum.Constants[i].GoName == parts[0] {
					c = &enum.Constants[i]
				}
			}
			if c == nil || seen[c.Value] {
				return bad("unknown or duplicate union variant")
			}
			seen[c.Value] = true
			v := Variant{GoName: c.GoName, Constant: enum.Name + "." + c.Name, Value: c.Value, Fields: []string{tag.Name}}
			if len(parts) == 2 && parts[1] != "" {
				for _, name := range strings.Split(parts[1], ",") {
					found := false
					for _, f := range d.Fields {
						if f.GoName == name {
							if f.Name == tag.Name || contains(v.Fields, f.Name) {
								return bad("duplicate variant field")
							}
							v.Fields = append(v.Fields, f.Name)
							found = true
						}
					}
					if !found {
						return bad("unknown union variant field " + name)
					}
				}
			}
			for _, f := range d.Fields {
				if !contains(v.Fields, f.Name) && !l.zeroAccepted(f.Type) {
					return bad("inactive variant field requires a valid Go zero value: " + f.GoName)
				}
			}
			d.Variants = append(d.Variants, v)
		}
		for _, c := range enum.Constants {
			if !seen[c.Value] {
				return bad(fmt.Sprintf("union is missing variant %s", c.GoName))
			}
		}
	}
	return nil
}
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (l *loader) namespaces() error {
	flat := map[string]bool{}
	for _, c := range l.api.Calls {
		if c.Receiver == "" && c.Namespace == "" {
			flat[c.Name] = true
		}
	}
	for _, c := range l.api.Calls {
		if c.Namespace != "" && flat[c.Namespace] {
			return fmt.Errorf("%s: namespace %s collides with a module function", c.Position, c.Namespace)
		}
	}
	return nil
}

func (l *loader) zeroAccepted(t *Type) bool {
	if t.Kind == "pointer" {
		return true
	}
	for _, d := range l.api.Declarations {
		if d.Name == t.Name && d.Validate != "" {
			if d.Validate != "enum" {
				return false
			}
			zero := "0"
			if t.Kind == "string" {
				zero = "\"\""
			}
			if t.Kind == "bool" {
				zero = "false"
			}
			for _, c := range d.Constants {
				if c.Value == zero {
					return true
				}
			}
			return false
		}
	}
	if t.Kind == "array" {
		return l.zeroAccepted(t.Elem)
	}
	if t.Kind == "record" {
		for _, d := range l.api.Declarations {
			if d.Name == t.Name {
				if d.unionSpec != "" {
					return false
				}
				for _, f := range d.Fields {
					if !l.zeroAccepted(f.Type) {
						return false
					}
				}
			}
		}
	}
	return true
}
