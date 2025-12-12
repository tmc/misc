package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"text/template"

	"go.starlark.net/lib/json"
	"go.starlark.net/lib/math"
	"go.starlark.net/lib/proto"
	"go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	goproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// loadStarlarkFuncs loads user-defined functions from funcs.star in the template directory.
// Each top-level function defined in the Starlark file becomes a Go template function.
// Proto types are automatically registered so Starlark can access them natively.
func loadStarlarkFuncs(templateDir string, types *protoregistry.Types) (template.FuncMap, error) {
	starFile := filepath.Join(templateDir, "funcs.star")
	content, err := os.ReadFile(starFile)
	if err != nil {
		return nil, err
	}

	// Create thread with proto support
	thread := &starlark.Thread{Name: "funcs.star"}

	// Register proto types so Starlark can access them via proto.file()
	// This uses the types we've collected from the proto files being generated
	proto.SetPool(thread, &typePool{types: types})

	// Predeclared modules available to all Starlark files
	predeclared := starlark.StringDict{
		"proto": proto.Module,
		"json":  json.Module,
		"math":  math.Module,
		"time":  time.Module,
	}

	// Parse and execute the Starlark file
	globals, err := starlark.ExecFile(thread, starFile, content, predeclared)
	if err != nil {
		return nil, fmt.Errorf("failed to execute starlark file: %w", err)
	}

	funcs := make(template.FuncMap)

	// Convert each Starlark function to a Go template function
	for name, value := range globals {
		if fn, ok := value.(*starlark.Function); ok {
			funcs[name] = makeTemplateFunc(thread, fn)
		}
	}

	return funcs, nil
}

// typePool adapts protoregistry.Types to the proto.DescriptorPool interface.
type typePool struct {
	types *protoregistry.Types
}

// FindFileByPath implements proto.DescriptorPool.
// It searches for a file descriptor that contains the requested path.
func (p *typePool) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	// The types registry doesn't directly support file lookup,
	// but we can search through registered message types
	var found protoreflect.FileDescriptor
	p.types.RangeMessages(func(mt protoreflect.MessageType) bool {
		fd := mt.Descriptor().ParentFile()
		if fd != nil && fd.Path() == path {
			found = fd
			return false // stop iteration
		}
		return true
	})
	if found != nil {
		return found, nil
	}
	// Also try extensions
	p.types.RangeExtensions(func(et protoreflect.ExtensionType) bool {
		fd := et.TypeDescriptor().ParentFile()
		if fd != nil && fd.Path() == path {
			found = fd
			return false
		}
		return true
	})
	if found != nil {
		return found, nil
	}
	return nil, fmt.Errorf("file not found: %s", path)
}

// makeTemplateFunc wraps a Starlark function as a Go template function.
// The returned function accepts any arguments, converts them to Starlark values,
// calls the Starlark function, and converts the result back to Go.
func makeTemplateFunc(thread *starlark.Thread, fn *starlark.Function) any {
	return func(args ...any) (any, error) {
		// Convert Go args to Starlark values
		starArgs := make([]starlark.Value, len(args))
		for i, arg := range args {
			v, err := goToStarlark(arg)
			if err != nil {
				return nil, fmt.Errorf("argument %d: %w", i, err)
			}
			starArgs[i] = v
		}

		// Call the Starlark function
		result, err := starlark.Call(thread, fn, starlark.Tuple(starArgs), nil)
		if err != nil {
			return nil, fmt.Errorf("starlark function %s: %w", fn.Name(), err)
		}

		// Convert result back to Go
		return starlarkToGo(result), nil
	}
}

// goToStarlark converts a Go value to a Starlark value.
func goToStarlark(v any) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}

	switch val := v.(type) {
	case bool:
		return starlark.Bool(val), nil
	case int:
		return starlark.MakeInt(val), nil
	case int32:
		return starlark.MakeInt(int(val)), nil
	case int64:
		return starlark.MakeInt64(val), nil
	case uint:
		return starlark.MakeUint(val), nil
	case uint32:
		return starlark.MakeUint(uint(val)), nil
	case uint64:
		return starlark.MakeUint64(val), nil
	case float32:
		return starlark.Float(val), nil
	case float64:
		return starlark.Float(val), nil
	case string:
		return starlark.String(val), nil
	case []byte:
		return starlark.String(val), nil
	case []string:
		list := make([]starlark.Value, len(val))
		for i, s := range val {
			list[i] = starlark.String(s)
		}
		return starlark.NewList(list), nil
	case []any:
		list := make([]starlark.Value, len(val))
		for i, elem := range val {
			sv, err := goToStarlark(elem)
			if err != nil {
				return nil, err
			}
			list[i] = sv
		}
		return starlark.NewList(list), nil
	case map[string]any:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			sv, err := goToStarlark(v)
			if err != nil {
				return nil, err
			}
			dict.SetKey(starlark.String(k), sv)
		}
		return dict, nil
	case *proto.Message:
		// Native proto.Message from lib/proto - pass through directly
		return val, nil
	case protoreflect.Message:
		// Convert to native lib/proto Message for full Starlark proto support
		return protoMessageToStarlark(val)
	case goproto.Message:
		// Go proto.Message - convert via protoreflect
		return protoMessageToStarlark(val.ProtoReflect())
	case protoreflect.Map:
		// Convert proto map to Starlark dict
		dict := starlark.NewDict(val.Len())
		val.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			sk, err := goToStarlark(k.Interface())
			if err != nil {
				return false
			}
			sv, err := goToStarlark(v.Interface())
			if err != nil {
				return false
			}
			dict.SetKey(sk, sv)
			return true
		})
		return dict, nil
	default:
		// Wrap structs and pointers in goStruct for attribute access
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return starlark.None, nil
			}
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			return &goStruct{v: v, rv: rv}, nil
		}
		// For slices, convert to Starlark list
		if rv.Kind() == reflect.Slice {
			list := make([]starlark.Value, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				elem, err := goToStarlark(rv.Index(i).Interface())
				if err != nil {
					return nil, err
				}
				list[i] = elem
			}
			return starlark.NewList(list), nil
		}
		// For maps, convert to Starlark dict
		if rv.Kind() == reflect.Map {
			dict := starlark.NewDict(rv.Len())
			iter := rv.MapRange()
			for iter.Next() {
				k, err := goToStarlark(iter.Key().Interface())
				if err != nil {
					return nil, err
				}
				v, err := goToStarlark(iter.Value().Interface())
				if err != nil {
					return nil, err
				}
				dict.SetKey(k, v)
			}
			return dict, nil
		}
		// Fallback: convert to string
		return starlark.String(fmt.Sprintf("%v", v)), nil
	}
}

// starlarkToGo converts a Starlark value back to a Go value.
func starlarkToGo(v starlark.Value) any {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = starlarkToGo(val.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range val.Items() {
			key := starlarkToGo(item[0])
			if k, ok := key.(string); ok {
				result[k] = starlarkToGo(item[1])
			}
		}
		return result
	case *proto.Message:
		// Return proto.Message as-is for template use
		return val
	case *goStruct:
		return val.v
	default:
		return val.String()
	}
}

// protoMessageToStarlark converts a protoreflect.Message to a native lib/proto Message.
// This enables full Starlark proto support including field access via dot notation.
func protoMessageToStarlark(pm protoreflect.Message) (starlark.Value, error) {
	// Marshal the Go proto message to binary
	data, err := goproto.Marshal(pm.Interface())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto message: %w", err)
	}
	// Unmarshal into a native lib/proto Message
	return proto.Unmarshal(pm.Descriptor(), data)
}

// goStruct wraps a Go struct for Starlark attribute access.
type goStruct struct {
	v  any
	rv reflect.Value
}

var _ starlark.Value = (*goStruct)(nil)
var _ starlark.HasAttrs = (*goStruct)(nil)

func (g *goStruct) String() string        { return fmt.Sprintf("%v", g.v) }
func (g *goStruct) Type() string          { return g.rv.Type().Name() }
func (g *goStruct) Freeze()               {}
func (g *goStruct) Truth() starlark.Bool  { return g.v != nil }
func (g *goStruct) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: %s", g.Type()) }

func (g *goStruct) Attr(name string) (starlark.Value, error) {
	rv := g.rv
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return starlark.None, nil
		}
		rv = rv.Elem()
	}

	// Try exported field first
	field := rv.FieldByName(name)
	if field.IsValid() && field.CanInterface() {
		return goToStarlark(field.Interface())
	}

	// Try method
	method := reflect.ValueOf(g.v).MethodByName(name)
	if method.IsValid() {
		// For no-arg methods that return a single value, call them
		mt := method.Type()
		if mt.NumIn() == 0 && mt.NumOut() >= 1 {
			results := method.Call(nil)
			if len(results) > 0 {
				return goToStarlark(results[0].Interface())
			}
		}
	}

	return starlark.None, nil
}

func (g *goStruct) AttrNames() []string {
	rv := g.rv
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	names := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.IsExported() {
			names = append(names, f.Name)
		}
	}
	return names
}
