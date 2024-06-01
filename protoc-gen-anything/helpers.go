package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/huandu/xstrings"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func (g *Generator) funcMap() template.FuncMap {
	return template.FuncMap{
		"string": func(i interface {
			String() string
		}) string {
			return i.String()
		},
		"json": func(v interface{}) string {
			a, err := json.Marshal(v)
			if err != nil {
				return err.Error()
			}
			return string(a)
		},
		"prettyjson": func(v interface{}) string {
			a, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return err.Error()
			}
			return string(a)
		},
		"splitArray": func(sep string, s string) []interface{} {
			var r []interface{}
			t := strings.Split(s, sep)
			for i := range t {
				if t[i] != "" {
					r = append(r, t[i])
				}
			}
			return r
		},
		"first": func(a []string) string {
			return a[0]
		},
		"last": func(a []string) string {
			return a[len(a)-1]
		},
		"concat": func(a string, b ...string) string {
			return strings.Join(append([]string{a}, b...), "")
		},
		"join": func(sep string, a ...string) string {
			return strings.Join(a, sep)
		},
		"upperFirst": func(s string) string {
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"lowerFirst": func(s string) string {
			return strings.ToLower(s[:1]) + s[1:]
		},
		"camelCase": func(s string) string {
			if len(s) > 1 {
				return xstrings.ToCamelCase(s)
			}

			return strings.ToUpper(s[:1])
		},
		"lowerCamelCase": func(s string) string {
			if len(s) > 1 {
				s = xstrings.ToCamelCase(s)
			}

			return strings.ToLower(s[:1]) + s[1:]
		},
		"upperCase": func(s string) string {
			return strings.ToUpper(s)
		},
		"kebabCase": func(s string) string {
			return strings.Replace(xstrings.ToSnakeCase(s), "_", "-", -1)
		},
		"contains": func(sub, s string) bool {
			return strings.Contains(s, sub)
		},
		"trimstr": func(cutset, s string) string {
			return strings.Trim(s, cutset)
		},
		"snakeCase":        xstrings.ToSnakeCase,
		"methodExtension":  g.helperMethodExtension,
		"messageExtension": g.helperMessageExtension,
		"fieldByName":      g.helperFieldByName,
	}
}

func (g *Generator) helperMethodExtension(method *protogen.Method, path string) any {
	options := method.Desc.Options().(*descriptorpb.MethodOptions)
	if options == nil {
		return nil
	}
	if err := g.marshalAndUnmarshalOptions(options); err != nil {
		log.Fatalf("%v", err)
	}
	extensions := g.getExtensions(options)
	return extensions[path]
}

func (g *Generator) helperMessageExtension(message *protogen.Message, path string) any {
	options := message.Desc.Options().(*descriptorpb.MessageOptions)
	if options == nil {
		return nil
	}
	if err := g.marshalAndUnmarshalOptions(options); err != nil {
		log.Fatalf("%v", err)
	}
	extensions := g.getExtensions(options)
	return extensions[path]
}

// gets a value of a field by name
func (g *Generator) helperFieldByName(message dynamicpb.Message, name string) any {
	fd := message.Descriptor().Fields().ByName(protoreflect.Name(name))
	return message.Get(fd)
}

func (g *Generator) marshalAndUnmarshalOptions(options proto.Message) error {
	b, err := proto.Marshal(options)
	if err != nil {
		return fmt.Errorf("Error marshalling options: %v", err)
	}
	err = proto.UnmarshalOptions{Resolver: g.types}.Unmarshal(b, options)
	if err != nil {
		return fmt.Errorf("Error unmarshalling options: %v", err)
	}
	return nil
}

func (g *Generator) getExtensions(options proto.Message) map[string]any {
	extensions := make(map[string]any)
	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() {
			extensions[string(fd.FullName())] = v.Interface()
		}
		return true
	})
	return extensions
}
