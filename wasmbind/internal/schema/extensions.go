package schema

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

func parseFeatures(s string) (map[string]bool, []string, error) {
	enabled := map[string]bool{}
	if s != "" {
		for _, f := range strings.Split(s, ",") {
			switch f {
			case "streams", "events", "imports", "errors":
			default:
				return nil, nil, fmt.Errorf("unknown feature %q", f)
			}
			if enabled[f] {
				return nil, nil, fmt.Errorf("duplicate feature %q", f)
			}
			enabled[f] = true
		}
	}
	var names []string
	for f := range enabled {
		names = append(names, f)
	}
	sort.Strings(names)
	return enabled, names, nil
}
func knownDirective(k string) bool {
	switch k {
	case "export", "record", "resource", "close", "read", "write", "finish", "cancelread", "cancelwrite", "next", "interrupt", "imports", "init", "error":
		return true
	}
	return false
}
func (l *loader) callSignature(obj *types.Func, sig *types.Signature) (*Call, error) {
	if sig.TypeParams().Len() != 0 || sig.Variadic() {
		return nil, l.diag(obj.Pos(), obj.Name(), "extension methods cannot be generic or variadic")
	}
	c := &Call{GoName: obj.Name(), Name: obj.Name(), Position: l.position(obj.Pos()), Params: []Param{}}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		if i == 0 && isContext(p.Type()) {
			c.Context = true
			continue
		}
		t, err := l.value(p.Type(), p.Pos())
		if err != nil {
			return nil, err
		}
		if containsResource(t) || containsDomain(l, t) {
			return nil, l.diag(p.Pos(), obj.Name(), "extension arguments must be copied values")
		}
		name := p.Name()
		if name == "" {
			name = fmt.Sprintf("arg%d", i)
		}
		c.Params = append(c.Params, Param{name, t})
	}
	return c, nil
}
func (l *loader) imports() error {
	for obj, d := range l.named {
		if d.Kind != "imports" {
			continue
		}
		iface := obj.Type().Underlying().(*types.Interface)
		if iface.NumEmbeddeds() != 0 {
			return l.diag(obj.Pos(), obj.Name(), "embedded import interfaces are unsupported")
		}
		d.ImportMethods = []*Call{}
		for j := 0; j < iface.NumExplicitMethods(); j++ {
			m := iface.ExplicitMethod(j)
			if !m.Exported() || !validName(m.Name()) {
				return l.diag(m.Pos(), m.Name(), "import methods must be public with nonreserved names")
			}
			sig := m.Type().(*types.Signature)
			c, err := l.callSignature(m, sig)
			if err != nil {
				return err
			}
			if !c.Context {
				return l.diag(m.Pos(), m.Name(), "import methods require context first")
			}
			r := sig.Results()
			if r.Len() < 1 || r.Len() > 2 || !isError(r.At(r.Len()-1).Type()) {
				return l.diag(m.Pos(), m.Name(), "import methods return error or (T,error)")
			}
			c.Error = true
			c.Receiver = d.Name
			c.Op = "import"
			if r.Len() == 2 {
				c.Result, err = l.value(r.At(0).Type(), r.At(0).Pos())
				if err != nil {
					return err
				}
				if containsResource(c.Result) || containsDomain(l, c.Result) {
					return l.diag(m.Pos(), m.Name(), "import results must be copied values")
				}
			}
			d.ImportMethods = append(d.ImportMethods, c)
		}
	}
	return nil
}
func (l *loader) initializer(obj *types.Func, sig *types.Signature, pos token.Pos, value string) error {
	if !l.features["imports"] || value != "" || sig.Recv() != nil || sig.TypeParams().Len() != 0 || sig.Variadic() {
		return l.diag(pos, obj.Name(), "init requires -features imports and a nongeneric function")
	}
	if l.api.Init != nil {
		return l.diag(pos, obj.Name(), "duplicate init")
	}
	var imports []*Decl
	for _, d := range l.api.Declarations {
		if d.Kind == "imports" {
			imports = append(imports, d)
		}
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Order < imports[j].Order })
	if sig.Params().Len() != len(imports)+1 || !isContext(sig.Params().At(0).Type()) || sig.Results().Len() != 1 || !isError(sig.Results().At(0).Type()) {
		return l.diag(pos, obj.Name(), "init requires context, all annotated interfaces in declaration order, and error")
	}
	c := &Call{Op: "init", GoName: obj.Name(), Context: true, Error: true, Position: l.position(pos), Params: []Param{}}
	for j, d := range imports {
		t := l.api.Pkg.Types.Scope().Lookup(d.GoName).Type()
		if !types.Identical(sig.Params().At(j+1).Type(), t) {
			return l.diag(pos, obj.Name(), "init interface parameters must follow declaration order")
		}
		c.Params = append(c.Params, Param{d.Name, &Type{Kind: "import", Name: d.Name, Go: l.goType(t)}})
	}
	l.api.Init = c
	return nil
}
func (l *loader) extensionMethod(obj *types.Func, sig *types.Signature, d *Decl, pos token.Pos, ds map[string]string, doc string) (bool, error) {
	var op, feature string
	for key := range ds {
		switch key {
		case "read", "write", "finish", "cancelread", "cancelwrite":
			op = key
			feature = "streams"
		case "next", "interrupt":
			op = key
			feature = "events"
		}
	}
	if op == "" {
		return false, nil
	}
	bad := func(msg string) (bool, error) { return true, l.diag(pos, obj.Name(), msg) }
	if !l.features[feature] {
		return bad(op + " requires -features " + feature)
	}
	if ds[op] != "" || d == nil || sig.TypeParams().Len() != 0 || sig.Variadic() {
		return bad(op + " requires a nongeneric resource method")
	}
	if d.Methods == nil {
		d.Methods = map[string]*Call{}
	}
	if d.Methods[op] != nil {
		return bad("duplicate " + op + " directive")
	}
	c := &Call{Op: op, GoName: obj.Name(), Name: "$" + op, Receiver: d.Name, ReceiverGo: d.GoName, Doc: doc, Position: l.position(pos), Params: []Param{}}
	isInt := func(t types.Type) bool { return types.Identical(t, types.Typ[types.Int]) }
	isBytes := func(t types.Type) bool { return types.Identical(t, types.NewSlice(types.Typ[types.Uint8])) }
	p, r := sig.Params(), sig.Results()
	switch op {
	case "read", "write":
		if p.Len() != 2 || !isContext(p.At(0).Type()) || !isBytes(p.At(1).Type()) || r.Len() != 2 || !isInt(r.At(0).Type()) || !isError(r.At(1).Type()) {
			return bad(op + " requires (context.Context,[]byte)(int,error)")
		}
		c.Context = true
		if op == "write" {
			c.Params = []Param{{"data", &Type{Kind: "bytes", Go: "[]byte"}}}
		} else {
			c.Params = []Param{{"size", &Type{Kind: "int", Go: "int"}}}
		}
	case "next":
		if p.Len() != 1 || !isContext(p.At(0).Type()) || r.Len() != 3 || !types.Identical(r.At(1).Type(), types.Typ[types.Bool]) || !isError(r.At(2).Type()) {
			return bad("next requires (context.Context)(T,bool,error)")
		}
		c.Context = true
		var err error
		c.Result, err = l.value(r.At(0).Type(), r.At(0).Pos())
		if err != nil {
			return true, err
		}
		if containsResource(c.Result) || containsDomain(l, c.Result) {
			return bad("event values must be copied values")
		}
	case "finish":
		if p.Len() != 1 || !isContext(p.At(0).Type()) || r.Len() != 1 || !isError(r.At(0).Type()) {
			return bad("finish requires (context.Context) error")
		}
		c.Context = true
	default:
		if p.Len() != 0 || r.Len() != 1 || !isError(r.At(0).Type()) {
			return bad(op + " requires () error")
		}
	}
	d.Methods[op] = c
	l.api.Calls = append(l.api.Calls, c)
	return true, nil
}
func (l *loader) finishExtensions() error {
	imports := 0
	for _, d := range l.api.Declarations {
		if d.Kind == "imports" {
			imports++
		}
		m := d.Methods
		if len(m) == 0 {
			continue
		}
		for a, b := range map[string]string{"read": "cancelread", "write": "cancelwrite", "next": "interrupt"} {
			if m[a] != nil && m[b] == nil {
				return fmt.Errorf("%s: %s: %s requires %s", d.Position, d.GoName, a, b)
			}
		}
		if m["write"] != nil && m["finish"] == nil {
			return fmt.Errorf("%s: %s: write requires finish", d.Position, d.GoName)
		}
		if m["read"] == nil && m["cancelread"] != nil || m["write"] == nil && (m["finish"] != nil || m["cancelwrite"] != nil) || m["next"] == nil && m["interrupt"] != nil {
			return fmt.Errorf("%s: %s: interruption hook has no matching direction", d.Position, d.GoName)
		}
		streams := m["read"] != nil || m["write"] != nil
		if streams && m["next"] != nil {
			return fmt.Errorf("%s: %s: streams and events cannot share one resource", d.Position, d.GoName)
		}
		for _, c := range l.api.Calls {
			if c.Receiver == d.Name && c.Op == "" && (streams || (m["next"] != nil && (c.Name == "next" || c.Name == "return" || c.Name == "throw"))) {
				return fmt.Errorf("%s: %s: ordinary method conflicts with resource extension", c.Position, c.GoName)
			}
		}
	}
	if l.features["imports"] && (l.api.Init == nil || imports == 0) {
		return fmt.Errorf("imports requires annotated interfaces and exactly one init function")
	}
	return nil
}
func containsDomain(l *loader, t *Type) bool {
	if t.Kind == "record" {
		for _, d := range l.api.Declarations {
			if d.Name == t.Name && d.Kind == "error" {
				return true
			}
		}
	}
	return t.Elem != nil && containsDomain(l, t.Elem)
}
