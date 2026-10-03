// Package generate emits standalone Go and JavaScript binding adapters.
package generate

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

//go:embed templates/*
var templates embed.FS

type emitter struct {
	api        *schema.API
	types      map[string]int
	converters bytes.Buffer
}

// Files renders a complete generation set without writing to disk.
func Files(a *schema.API) (map[string][]byte, error) {
	e := &emitter{api: a, types: map[string]int{}}
	var prep bytes.Buffer
	fmt.Fprintln(&prep, "func (b *runtimeBridge) prepare(method, receiver int, args js.Value, i *invocation)(func()(js.Value,error),error) {\n budget:= &valueBudget{limits:b.limits}; _=budget\n switch method {")
	for _, c := range a.Calls {
		fmt.Fprintf(&prep, "case %d:\nif args.Length()!=%d {return nil,failure(\"argument\",\"wrong argument count\")}\n", c.ID, len(c.Params))
		if c.Receiver != "" {
			fmt.Fprintf(&prep, "r,err:=b.hold(i,receiver,%q);if err!=nil{return nil,err};recv:=r.ptr.(*api.%s);_ = recv\nb.queue(r,i,%q)\n", c.Receiver, c.ReceiverGo, c.Op)
		}
		var args []string
		if c.Context {
			args = append(args, "i.ctx")
		}
		for j, p := range c.Params {
			n := e.converter(p.Type)
			fmt.Fprintf(&prep, "a%d,err:=b.decode%d(args.Index(%d),i,budget,0);if err!=nil{return nil,err}\n", j, n, j)
			args = append(args, fmt.Sprintf("a%d", j))
		}
		target := "api." + c.GoName
		if c.Receiver != "" {
			target = "recv." + c.GoName
		}
		if len(c.TypeArgs) > 0 {
			target += "[" + strings.Join(c.TypeArgs, ",") + "]"
		}
		fmt.Fprintln(&prep, "return func()(js.Value,error) {")
		if e.extensionCall(&prep, c) {
			fmt.Fprintln(&prep, "},nil")
			continue
		}
		lhs := ""
		if c.Result != nil {
			lhs = "value"
		}
		if c.Error {
			if lhs != "" {
				lhs += ","
			}
			lhs += "callErr"
		}
		if lhs != "" {
			lhs += " := "
		}
		fmt.Fprintf(&prep, "%s%s(%s)\n", lhs, target, strings.Join(args, ","))
		if c.Result != nil && hasResource(c.Result) {
			n := e.converter(c.Result)
			fmt.Fprintf(&prep, "adoptErr:=b.adopt%d(value,i)\n", n)
		}
		if c.Error {
			fmt.Fprintln(&prep, "if callErr!=nil {return js.Undefined(),callErr}")
		}
		if c.Result != nil && hasResource(c.Result) {
			fmt.Fprintln(&prep, "if adoptErr!=nil {return js.Undefined(),adoptErr}")
		}
		if c.Result != nil {
			n := e.converter(c.Result)
			fmt.Fprintf(&prep, "return b.encode%d(value,i,&valueBudget{limits:b.limits},0)\n", n)
		} else {
			fmt.Fprintln(&prep, "return js.Undefined(),nil")
		}
		fmt.Fprintln(&prep, "},nil")
	}
	fmt.Fprintln(&prep, "default:return nil,failure(\"argument\",\"unknown export\")\n}\n}")
	rawExtensions, _ := templates.ReadFile("templates/extensions.go.txt")
	extensionSource := string(rawExtensions) + e.extensions()
	var imports []string
	for path, alias := range a.Imports {
		imports = append(imports, fmt.Sprintf("%s %q", alias, path))
	}
	sort.Strings(imports)
	if !strings.Contains(prep.String()+extensionSource+e.converters.String(), "api.") {
		imports = []string{fmt.Sprintf("_ %q", a.Package)}
	}
	raw, _ := templates.ReadFile("templates/bridge.go.txt")
	source := strings.NewReplacer("__IMPORTS__", strings.Join(imports, "\n"), "__FINGERPRINT__", a.Fingerprint, "__CONVERTERS__", e.converters.String(), "__PREPARE__", prep.String()+extensionSource).Replace(string(raw))
	goSource, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("format generated bridge: %w", err)
	}
	if a.Backend == "abi" {
		goSource, err = abiSource(a, goSource)
		if err != nil {
			return nil, err
		}
	}
	manifest, _ := json.MarshalIndent(a.Manifest, "", "  ")
	manifest = append(manifest, '\n')
	jsSource, _ := templates.ReadFile("templates/bindings.js")
	jsExtensions, _ := templates.ReadFile("templates/extensions.js")
	compact, _ := json.Marshal(a.Manifest)
	if a.Backend == "abi" {
		raw, _ := templates.ReadFile("templates/abi.js")
		jsExtensions = append(jsExtensions, raw...)
		jsSource = []byte(strings.ReplaceAll(string(jsSource), "const go = new Go();", "if(schema.backend==='abi')return loadABI(source,limits,Go);\n  const go = new Go();"))
	}
	return map[string][]byte{"main.go": goSource, "bindings.js": []byte(strings.NewReplacer("__SCHEMA__", string(compact), "__ARTIFACT_ID__", fmt.Sprintf("%q", a.BuildIdentity), "__EXTENSIONS__", string(jsExtensions), "__DOMAINS__", domainClasses(a)).Replace(string(jsSource))), "bindings.d.ts": []byte(declarations(a)), "bindings.json": manifest}, nil
}
func (e *emitter) decl(name string) *schema.Decl {
	for _, d := range e.api.Declarations {
		if d.Name == name {
			return d
		}
	}
	return nil
}
func (e *emitter) converter(t *schema.Type) int {
	key := t.Go
	if n, ok := e.types[key]; ok {
		return n
	}
	n := len(e.types)
	e.types[key] = n
	var dec, enc bytes.Buffer
	fmt.Fprintf(&dec, "func (b *runtimeBridge) decode%d(v js.Value,i *invocation,q *valueBudget,depth int)(%s,error) {\nvar out %s\nif err:=q.charge(0,0,depth);err!=nil{return out,err}\n", n, t.Go, t.Go)
	fmt.Fprintf(&enc, "func (b *runtimeBridge) encode%d(v %s,i *invocation,q *valueBudget,depth int)(js.Value,error) {\nif err:=q.charge(0,0,depth);err!=nil{return js.Undefined(),err}\n", n, t.Go)
	switch t.Kind {
	case "bool":
		fmt.Fprintf(&dec, "if v.Type()!=js.TypeBoolean{return out,failure(\"argument\",\"expected boolean\")};return %s(v.Bool()),nil\n", t.Go)
		fmt.Fprintln(&enc, "return js.ValueOf(bool(v)),nil")
	case "string":
		fmt.Fprintf(&dec, "if v.Type()!=js.TypeString{return out,failure(\"argument\",\"expected string\")};s:=v.String();if err:=q.charge(len(s),0,depth);err!=nil{return out,err};return %s(s),nil\n", t.Go)
		fmt.Fprintln(&enc, "s:=string(v);if !utf8.ValidString(s){return js.Undefined(),failure(\"argument\",\"invalid UTF-8 result\")};if err:=q.charge(len(s),0,depth);err!=nil{return js.Undefined(),err};return js.ValueOf(s),nil")
	case "int64", "uint64":
		parse, formatFn, bits := "ParseInt", "FormatInt", 64
		if t.Kind == "uint64" {
			parse = "ParseUint"
			formatFn = "FormatUint"
		}
		fmt.Fprintf(&dec, "x,err:=strconv.%s(v.String(),10,%d);if err!=nil{return out,failure(\"argument\",\"bigint out of range\")};return %s(x),nil\n", parse, bits, t.Go)
		fmt.Fprintf(&enc, "return js.ValueOf(strconv.%s(%s(v),10)),nil\n", formatFn, t.Kind)
	case "bytes":
		fmt.Fprintf(&dec, "if !v.InstanceOf(js.Global().Get(\"Uint8Array\")){return out,failure(\"argument\",\"expected Uint8Array\")};n:=v.Length();if err:=q.charge(n,n,depth);err!=nil{return out,err};out=make(%s,n);js.CopyBytesToGo(out,v);return out,nil\n", t.Go)
		fmt.Fprintln(&enc, "if err:=q.charge(len(v),len(v),depth);err!=nil{return js.Undefined(),err};out:=js.Global().Get(\"Uint8Array\").New(len(v));js.CopyBytesToJS(out,v);return out,nil")
	case "slice", "array":
		child := e.converter(t.Elem)
		fmt.Fprintln(&dec, "n:=v.Length();if err:=q.charge(0,n,depth);err!=nil{return out,err}")
		if t.Kind == "slice" {
			fmt.Fprintf(&dec, "out=make(%s,n)\n", t.Go)
		} else {
			fmt.Fprintf(&dec, "if n!=%d{return out,failure(\"argument\",\"array has wrong length\")}\n", t.Length)
		}
		fmt.Fprintf(&dec, "for j:=0;j<n;j++ {x,err:=b.decode%d(v.Index(j),i,q,depth+1);if err!=nil{return out,err};out[j]=x};return out,nil\n", child)
		fmt.Fprintln(&enc, "if err:=q.charge(0,len(v),depth);err!=nil{return js.Undefined(),err};out:=js.Global().Get(\"Array\").New(len(v))")
		fmt.Fprintf(&enc, "for j,x:=range v {v,err:=b.encode%d(x,i,q,depth+1);if err!=nil{return js.Undefined(),err};out.SetIndex(j,v)};return out,nil\n", child)
	case "pointer":
		child := e.converter(t.Elem)
		fmt.Fprintf(&dec, "if v.IsNull(){return nil,nil};x,err:=b.decode%d(v,i,q,depth+1);if err!=nil{return nil,err};return &x,nil\n", child)
		fmt.Fprintf(&enc, "if v==nil{return js.Null(),nil};return b.encode%d(*v,i,q,depth+1)\n", child)
	case "record":
		d := e.decl(t.Name)
		fmt.Fprintf(&dec, "if err:=q.charge(0,%d,depth);err!=nil{return out,err}\n", len(d.Fields))
		fmt.Fprintf(&enc, "if err:=q.charge(0,%d,depth);err!=nil{return js.Undefined(),err};out:=js.Global().Get(\"Object\").Call(\"create\",js.Null())\n", len(d.Fields))
		for j, f := range d.Fields {
			child := e.converter(f.Type)
			fmt.Fprintf(&dec, "x%d,err:=b.decode%d(v.Get(%q),i,q,depth+1);if err!=nil{return out,err};out.%s=x%d\n", j, child, f.Name, f.GoName, j)
			fmt.Fprintf(&enc, "x%d,err:=b.encode%d(v.%s,i,q,depth+1);if err!=nil{return js.Undefined(),err};out.Set(%q,x%d)\n", j, child, f.GoName, f.Name, j)
		}
		fmt.Fprintln(&dec, "return out,nil")
		fmt.Fprintln(&enc, "return out,nil")
	case "resource":
		d := e.decl(t.Name)
		fmt.Fprintf(&dec, "if v.IsNull(){return nil,nil};r,err:=b.hold(i,v.Int(),%q);if err!=nil{return nil,err};return r.ptr.(%s),nil\n", t.Name, t.Go)
		closeStmt := "v." + d.Close + "();return nil"
		if d.CloseError {
			closeStmt = "return v." + d.Close + "()"
		}
		fmt.Fprintf(&enc, "if v==nil{return js.Null(),nil};return b.register(i,weak.Make(v),v,%q,func()error{%s},%s)\n", t.Name, closeStmt, e.hookMap(d))
	default:
		fmt.Fprintf(&dec, "if v.Type()!=js.TypeNumber{return out,failure(\"argument\",\"expected number\")};return %s(v.Float()),nil\n", t.Go)
		if t.Kind == "float32" || t.Kind == "float64" {
			fmt.Fprintln(&enc, "if math.IsNaN(float64(v)) || math.IsInf(float64(v),0){return js.Undefined(),failure(\"argument\",\"non-finite result\")}")
		}
		if t.Kind == "int" || t.Kind == "uint" {
			fmt.Fprintf(&enc, "if %s(float64(v))!=v {return js.Undefined(),failure(\"argument\",\"integer result is not exactly representable as number\")}\n", t.Go)
		}
		fmt.Fprintln(&enc, "return js.ValueOf(float64(v)),nil")
	}
	fmt.Fprintln(&dec, "}")
	fmt.Fprintln(&enc, "}")
	e.converters.Write(dec.Bytes())
	e.converters.Write(enc.Bytes())
	if hasResource(t) {
		fmt.Fprintf(&e.converters, "func (b *runtimeBridge) adopt%d(v %s,i *invocation)error {\n", n, t.Go)
		if t.Kind == "resource" {
			fmt.Fprintf(&e.converters, "_,err:=b.encode%d(v,i,&valueBudget{limits:b.limits},0);return err\n", n)
		} else {
			child := e.converter(t.Elem)
			fmt.Fprintf(&e.converters, "var err error;for _,x:=range v {next:=b.adopt%d(x,i);if err==nil{err=next}};return err\n", child)
		}
		fmt.Fprintln(&e.converters, "}")
	}
	return n
}
func tsType(t *schema.Type) string {
	switch t.Kind {
	case "bool":
		return "boolean"
	case "string":
		return "string"
	case "int64", "uint64":
		return "bigint"
	case "bytes":
		return "Uint8Array"
	case "record":
		return t.Name
	case "resource":
		return t.Name + " | null"
	case "pointer":
		return tsType(t.Elem) + " | null"
	case "array", "slice":
		return "Array<" + tsType(t.Elem) + ">"
	default:
		return "number"
	}
}
func doc(b *bytes.Buffer, s string) {
	if s != "" {
		fmt.Fprintf(b, "/** %s */\n", strings.ReplaceAll(s, "*/", "* /"))
	}
}
func declarations(a *schema.API) string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by wasmbind. DO NOT EDIT.\ndeclare const resourceBrand: unique symbol;\nexport interface CallOptions { signal?: AbortSignal }\nexport interface LoadOptions { startupTimeoutMs?: number; maxPendingCalls?: number; maxResources?: number; maxValueBytes?: number; maxValueDepth?: number }\nexport class WasmBindError extends Error { readonly code: string; constructor(code: string, message: string) }")
	if hasFeature(a, "streams") {
		fmt.Fprintln(&b, "export interface LoadOptions { streamChunkBytes?: number; streamHighWaterBytes?: number }")
	}
	if hasFeature(a, "imports") {
		fmt.Fprintln(&b, "export interface LoadOptions { imports: Imports; maxPendingImports?: number }\nexport interface Imports {")
		for _, d := range a.Declarations {
			if d.Kind != "imports" {
				continue
			}
			fmt.Fprintf(&b, "%s: {\n", d.Name)
			for _, c := range d.ImportMethods {
				var params []string
				for j, p := range c.Params {
					params = append(params, fmt.Sprintf("arg%d: %s", j, tsType(p.Type)))
				}
				params = append(params, "options: {readonly signal: AbortSignal}")
				result := "void"
				if c.Result != nil {
					result = tsType(c.Result)
				}
				fmt.Fprintf(&b, "%s(%s): %s | Promise<%s>;\n", c.Name, strings.Join(params, ", "), result, result)
			}
			fmt.Fprintln(&b, "};")
		}
		fmt.Fprintln(&b, "}")
	}
	method := func(c *schema.Call) {
		doc(&b, c.Doc)
		var p []string
		for _, arg := range c.Params {
			name := arg.Name
			switch name {
			case "delete", "await", "yield", "class", "new", "default", "switch", "throw", "try", "catch", "finally", "export", "import", "extends", "super", "this", "null", "true", "false", "with", "in", "instanceof", "typeof", "void", "enum", "implements", "private", "public", "protected", "static", "let", "const", "function", "debugger", "var", "do", "while", "of":
				name = "arg" + fmt.Sprint(len(p))
			}
			p = append(p, name+": "+tsType(arg.Type))
		}
		if c.Context {
			p = append(p, "options?: CallOptions")
		}
		result := "void"
		if c.Result != nil {
			result = tsType(c.Result)
		}
		fmt.Fprintf(&b, "%s(%s): Promise<%s>;\n", c.Name, strings.Join(p, ", "), result)
	}
	for _, d := range a.Declarations {
		doc(&b, d.Doc)
		if d.Kind == "imports" {
			continue
		}
		if d.Kind == "error" {
			fmt.Fprintf(&b, "export interface %sData {\n", d.Name)
			for _, f := range d.Fields {
				fmt.Fprintf(&b, "%s: %s;\n", f.Name, tsType(f.Type))
			}
			fmt.Fprintf(&b, "}\nexport class %s extends Error {readonly code: 'go-error';readonly domain: %q;readonly data: %sData;constructor(message: string,data: %sData)}\nexport function is%s(value: unknown): value is %s;\n", d.Name, d.Domain, d.Name, d.Name, d.Name, d.Name)
			continue
		}
		extends := ""
		if c := d.Methods["next"]; c != nil {
			extends = " extends AsyncIterableIterator<" + tsType(c.Result) + ">"
		}
		fmt.Fprintf(&b, "export interface %s%s {\n", d.Name, extends)
		if d.Kind == "record" {
			for _, f := range d.Fields {
				fmt.Fprintf(&b, "%s: %s;\n", f.Name, tsType(f.Type))
			}
		} else {
			fmt.Fprintf(&b, "readonly [resourceBrand]: %q;\n", d.Name)
			if d.Methods["read"] != nil {
				fmt.Fprintln(&b, "readonly readable: ReadableStream<Uint8Array>;")
			}
			if d.Methods["write"] != nil {
				fmt.Fprintln(&b, "readonly writable: WritableStream<Uint8Array>;")
			}
			for _, c := range a.Calls {
				if c.Receiver == d.Name && c.Op == "" {
					method(c)
				}
			}
			fmt.Fprintln(&b, "dispose(): Promise<void>;")
		}
		fmt.Fprintln(&b, "}")
	}
	fmt.Fprintln(&b, "export interface Module {")
	for _, c := range a.Calls {
		if c.Receiver == "" && c.Op == "" {
			method(c)
		}
	}
	optional := "?"
	if hasFeature(a, "imports") {
		optional = ""
	}
	fmt.Fprintf(&b, "dispose(): Promise<void>;\n}\nexport function load(source: URL | Response | BufferSource | WebAssembly.Module, options%s: LoadOptions): Promise<Module>;\n", optional)
	return b.String()
}

func hasResource(t *schema.Type) bool {
	return t.Kind == "resource" || (t.Elem != nil && hasResource(t.Elem))
}

func hasFeature(a *schema.API, name string) bool {
	for _, f := range a.Features {
		if f == name {
			return true
		}
	}
	return false
}
