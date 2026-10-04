package schema

import (
	"encoding/json"
	"go/constant"
	"go/types"
	"math"
	"strconv"
	"unicode/utf8"
)

// fieldDefault accepts constants only: decoding an omitted field must never run
// application code. The schema carries the normalized JavaScript literal.
func (l *loader) fieldDefault(f *types.Var, expression string) (string, error) {
	basic, ok := f.Type().Underlying().(*types.Basic)
	if !ok {
		return "", l.diag(f.Pos(), f.Name(), "default requires a basic copied value")
	}
	v, err := types.Eval(l.api.Pkg.Fset, l.api.Pkg.Types, f.Pos(), expression)
	if err != nil || v.Value == nil || !types.AssignableTo(v.Type, f.Type()) {
		return "", l.diag(f.Pos(), f.Name(), "default must be a constant assignable to its field")
	}
	value := v.Value
	if basic.Info()&types.IsInteger != 0 {
		bits := 64
		unsigned := basic.Info()&types.IsUnsigned != 0
		switch basic.Kind() {
		case types.Int8, types.Uint8:
			bits = 8
		case types.Int16, types.Uint16:
			bits = 16
		case types.Int32, types.Uint32:
			bits = 32
		}
		text := constant.ToInt(value).ExactString()
		var rangeErr error
		if unsigned {
			_, rangeErr = strconv.ParseUint(text, 10, bits)
		} else {
			_, rangeErr = strconv.ParseInt(text, 10, bits)
		}
		if rangeErr != nil {
			return "", l.diag(f.Pos(), f.Name(), "default integer is out of range")
		}
	}
	switch value.Kind() {
	case constant.String:
		str := constant.StringVal(value)
		if !utf8.ValidString(str) {
			return "", l.diag(f.Pos(), f.Name(), "default must be valid UTF-8")
		}
		b, _ := json.Marshal(str)
		return string(b), nil
	case constant.Bool:
		return value.ExactString(), nil
	case constant.Int:
		if basic.Kind() == types.Int64 || basic.Kind() == types.Uint64 {
			return value.ExactString() + "n", nil
		}
		number, exact := constant.Float64Val(constant.ToFloat(value))
		if !exact {
			return "", l.diag(f.Pos(), f.Name(), "default integer must be exactly representable as number")
		}
		return strconv.FormatFloat(number, 'g', -1, 64), nil
	case constant.Float:
		number, _ := constant.Float64Val(value)
		if math.IsInf(number, 0) || (basic.Kind() == types.Float32 && math.IsInf(float64(float32(number)), 0)) {
			return "", l.diag(f.Pos(), f.Name(), "default float is out of range")
		}
		return strconv.FormatFloat(number, 'g', -1, 64), nil
	}
	return "", l.diag(f.Pos(), f.Name(), "unsupported default constant")
}

// inputs marks only record shapes whose input differs from their output. The
// loader has already rejected recursive record graphs before this pass.
func (l *loader) inputs() error {
	declarations := make(map[string]*Decl)
	for _, d := range l.api.Declarations {
		declarations[d.Name] = d
	}
	done := make(map[string]bool)
	var markType func(*Type) bool
	var markRecord func(*Decl) bool
	markRecord = func(d *Decl) bool {
		if done[d.Name] {
			return d.Input
		}
		done[d.Name] = true
		for _, f := range d.Fields {
			changed := markType(f.Type)
			d.Input = d.Input || f.Optional || changed
		}
		return d.Input
	}
	markType = func(t *Type) bool {
		if t == nil {
			return false
		}
		if t.Kind == "record" {
			d := declarations[t.Name]
			if d == nil {
				return false
			}
			t.Input = markRecord(d)
			return t.Input
		}
		if t.Elem != nil {
			return markType(t.Elem)
		}
		return false
	}
	for _, d := range l.api.Declarations {
		if d.Kind == "record" {
			markRecord(d)
		}
		for _, c := range d.ImportMethods {
			for _, p := range c.Params {
				markType(p.Type)
			}
			markType(c.Result)
		}
	}
	for _, c := range l.api.Calls {
		for _, p := range c.Params {
			markType(p.Type)
		}
		markType(c.Result)
	}
	for _, d := range l.api.Declarations {
		if d.Kind == "record" && d.Input {
			obj := l.api.Pkg.Types.Scope().Lookup(d.GoName)
			if err := l.name("types", d.Name+"Input", obj.Pos(), d.GoName); err != nil {
				return err
			}
		}
	}
	return nil
}
