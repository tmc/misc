package schema

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"unicode"
	"unicode/utf8"
)

func hasDirective(ds map[string]string, name string) bool { _, ok := ds[name]; return ok }

// memberName lowers the first word, preserving the boundary between an acronym
// and the next word: HTTPServer becomes httpServer and URL becomes url.
// A plural acronym such as URLs becomes urls.
func (l *loader) memberName(name string) string {
	if l.api.Naming != "camelCase" {
		return name
	}
	r := []rune(name)
	n := 0
	for n < len(r) && unicode.IsUpper(r[n]) {
		n++
	}
	if n > 1 && n < len(r) && unicode.IsLower(r[n]) && !(n == len(r)-1 && r[n] == 's') {
		n--
	}
	for i := 0; i < n; i++ {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

func (l *loader) packagePolicy() error {
	p := l.api.Pkg
	for _, f := range p.Syntax {
		ds, _, err := directives(f.Doc)
		if err != nil {
			return l.diag(f.Pos(), "package", err.Error())
		}
		for key, v := range ds {
			if key != "naming" || v != "camelCase" {
				return l.diag(f.Pos(), "package", "package directive must be naming camelCase")
			}
			if l.api.Naming != "" {
				return l.diag(f.Pos(), "package", "duplicate naming directive")
			}
			l.api.Naming = v
		}
		var fieldErr error
		ast.Inspect(f, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok {
				return true
			}
			ds, text, err := directives(field.Doc)
			if err != nil {
				fieldErr = l.diag(field.Pos(), "field", err.Error())
				return false
			}
			if text == "" && field.Comment != nil {
				_, text, _ = directives(field.Comment)
			}
			for _, name := range field.Names {
				l.docs[name.Pos()] = text
				l.methodDirectives[name.Pos()] = ds
			}
			return true
		})
		if fieldErr != nil {
			return fieldErr
		}
	}
	return nil
}

func (l *loader) scalars() error {
	p := l.api.Pkg
	for obj, d := range l.named {
		if d.Kind != "scalar" && d.Kind != "enum" {
			continue
		}
		n := obj.Type().(*types.Named)
		basic, ok := n.Underlying().(*types.Basic)
		if !ok {
			return l.diag(obj.Pos(), obj.Name(), "scalar and enum require a supported basic type")
		}
		t, err := l.value(basic, obj.Pos())
		if err != nil {
			return err
		}
		d.Value = t
		if d.Kind == "enum" && (basic.Info()&types.IsFloat) != 0 {
			return l.diag(obj.Pos(), obj.Name(), "enum requires string, bool, or integer values")
		}
		if d.Validate != "" && d.Validate != "enum" {
			if l.api.Backend == "abi" {
				return l.diag(obj.Pos(), obj.Name(), "Go scalar validators require syscalljs; application code cannot execute inside ABI submission")
			}
			v, ok := p.Types.Scope().Lookup(d.Validate).(*types.Func)
			if !ok || !v.Exported() {
				return l.diag(obj.Pos(), obj.Name(), "validator requires a public function")
			}
			sig := v.Type().(*types.Signature)
			if sig.Recv() != nil || sig.TypeParams().Len() != 0 || sig.Variadic() || sig.Params().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), obj.Type()) || sig.Results().Len() != 1 || !isError(sig.Results().At(0).Type()) {
				return l.diag(v.Pos(), v.Name(), "validator requires func(T) error for its annotated scalar")
			}
		}
		if d.Kind != "enum" {
			continue
		}
		seen := map[string]bool{}
		for _, f := range p.Syntax {
			for _, node := range f.Decls {
				g, ok := node.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range g.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					doc := v.Doc
					if doc == nil {
						doc = g.Doc
					}
					_, text, err := directives(doc)
					if err != nil {
						return l.diag(v.Pos(), obj.Name(), err.Error())
					}
					for _, ident := range v.Names {
						c, ok := p.TypesInfo.Defs[ident].(*types.Const)
						if !ok || !c.Exported() || !types.Identical(c.Type(), obj.Type()) {
							continue
						}
						name := l.memberName(c.Name())
						if !validName(name) || seen[name] {
							return l.diag(c.Pos(), c.Name(), "invalid, reserved, or duplicate enum member name")
						}
						seen[name] = true
						value := c.Val().ExactString()
						if t.Kind == "int" || t.Kind == "uint" {
							f, exact := constant.Float64Val(constant.ToFloat(c.Val()))
							if !exact || constant.Compare(constant.MakeFloat64(f), token.NEQ, c.Val()) {
								return l.diag(c.Pos(), c.Name(), "enum integer constant is not exactly representable as JavaScript number; use int64 or uint64")
							}
						}
						if c.Val().Kind() == constant.String {
							valueString := constant.StringVal(c.Val())
							if !utf8.ValidString(valueString) {
								return l.diag(c.Pos(), c.Name(), "enum string constant must be valid UTF-8")
							}
							b, _ := json.Marshal(valueString)
							value = string(b)
						}
						d.Constants = append(d.Constants, Constant{Name: name, GoName: c.Name(), Value: value, Doc: text})
					}
				}
			}
		}
		if len(d.Constants) == 0 {
			return l.diag(obj.Pos(), obj.Name(), "enum requires at least one exported typed constant")
		}
	}
	return nil
}

func (l *loader) defaultFactory(obj *types.TypeName, d *Decl) error {
	v, ok := l.api.Pkg.Types.Scope().Lookup(d.Default).(*types.Func)
	if !ok || !v.Exported() {
		return l.diag(obj.Pos(), obj.Name(), "optional imports require a public default factory")
	}
	sig := v.Type().(*types.Signature)
	if sig.Recv() != nil || sig.TypeParams().Len() != 0 || sig.Variadic() || sig.Params().Len() != 0 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), obj.Type()) {
		return l.diag(v.Pos(), v.Name(), "default factory requires func() T for its annotated imports interface")
	}
	return nil
}
