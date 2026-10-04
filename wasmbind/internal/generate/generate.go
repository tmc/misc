// Package generate emits standalone Go and JavaScript binding adapters.
package generate

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"go/format"
	"regexp"
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
			fmt.Fprintf(&prep, "r,err:=b.hold(i,receiver,%q);if err!=nil{return nil,err};recv:=r.ptr.(*api.%s);_ = recv\n", c.Receiver, c.ReceiverGo)
			concurrent := false
			for _, d := range a.Declarations {
				if d.Name == c.Receiver {
					concurrent = d.Concurrent
					break
				}
			}
			if !concurrent || c.Op != "" {
				fmt.Fprintf(&prep, "b.queue(r,i,%q)\n", c.Op)
			}
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
		if c.NonNull {
			fmt.Fprintln(&prep, `if value==nil {return js.Undefined(),failure("contract","nonnull export returned nil")}`)
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
	return map[string][]byte{"main.go": goSource, "bindings.js": []byte(strings.NewReplacer("__SCHEMA__", string(compact), "__ARTIFACT_ID__", fmt.Sprintf("%q", a.BuildIdentity), "__EXTENSIONS__", string(jsExtensions), "__DOMAINS__", domainClasses(a)+enumExports(a)).Replace(string(jsSource))), "bindings.d.ts": []byte(declarations(a)), "bindings.json": manifest, "API.md": APIReference(a)}, nil
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
	if d := e.decl(t.Name); d != nil && (d.Kind == "scalar" || d.Kind == "enum") {
		child := e.converter(d.Value)
		fmt.Fprintf(&dec, "raw,err:=b.decode%d(v,i,q,depth);if err!=nil{return out,err};out=%s(raw)\n", child, t.Go)
		e.validate(&dec, d, "out", "return out,err", true)
		fmt.Fprintln(&dec, "return out,nil")
		e.validate(&enc, d, "v", "return js.Undefined(),err", false)
		fmt.Fprintf(&enc, "return b.encode%d(%s(v),i,q,depth)\n", child, d.Value.Go)
	} else {
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
	// Named primitive values retain their domain name at the TypeScript boundary.
	if t.Name != "" && t.Kind != "resource" && t.Kind != "pointer" && t.Kind != "array" && t.Kind != "slice" {
		return t.Name
	}
	switch t.Kind {
	case "bool":
		return "boolean"
	case "string":
		return "string"
	case "int64", "uint64":
		return "bigint"
	case "bytes":
		return "Uint8Array"
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

func writeDoc(b *bytes.Buffer, indent, s string) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "*/", "* /"))
	if s == "" {
		return
	}
	lines := strings.Split(s, "\n")
	if len(lines) == 1 {
		fmt.Fprintf(b, "%s/** %s */\n", indent, s)
		return
	}
	fmt.Fprintln(b, indent+"/**")
	for _, line := range lines {
		fmt.Fprintln(b, strings.TrimRight(indent+" * "+line, " "))
	}
	fmt.Fprintln(b, indent+" */")
}

// declarationDoc changes complete Go identifiers in comments to their generated
// names. Ambiguous names are left intact; a field named Value in one record must
// not rewrite prose describing a different record's Value field.
func declarationDoc(a *schema.API, s string) string {
	names := make(map[string]string)
	ambiguous := make(map[string]bool)
	add := func(old, name string) {
		if old == "" {
			return
		}
		if previous, ok := names[old]; ok && previous != name {
			ambiguous[old] = true
		}
		names[old] = name
	}
	for _, d := range a.Declarations {
		add(d.GoName, d.Name)
		for _, f := range d.Fields {
			add(f.GoName, f.Name)
		}
		for _, c := range d.ImportMethods {
			add(c.GoName, c.Name)
		}
		for _, c := range d.Constants {
			add(c.GoName, c.Name)
		}
	}
	for _, c := range a.Calls {
		if c.Op == "" {
			add(c.GoName, c.Name)
		}
	}
	return identifiers.ReplaceAllStringFunc(s, func(word string) string {
		if name, ok := names[word]; ok && !ambiguous[word] {
			return name
		}
		return word
	})
}

var identifiers = regexp.MustCompile(`[\pL_$][\pL\pN_$]*`)

func parameterName(name string, index int) string {
	switch name {
	case "", "delete", "await", "yield", "class", "new", "default", "switch", "throw", "try", "catch", "finally", "export", "import", "extends", "super", "this", "null", "true", "false", "with", "in", "instanceof", "typeof", "void", "enum", "implements", "private", "public", "protected", "static", "let", "const", "function", "debugger", "var", "do", "while", "of":
		return fmt.Sprintf("arg%d", index)
	}
	return name
}

func callSignature(c *schema.Call, imported bool) string {
	var params []string
	reserved := make(map[string]bool)
	for _, p := range c.Params {
		reserved[p.Name] = true
	}
	used := make(map[string]bool)
	if imported || c.Context {
		used["options"] = true
		reserved["options"] = true
	}
	for i, p := range c.Params {
		name := parameterName(p.Name, i)
		if name != p.Name || used[name] {
			base := name
			if name == "options" {
				base = "optionsArg"
			}
			name = base
			for suffix := 2; used[name] || reserved[name]; suffix++ {
				name = fmt.Sprintf("%s_%d", base, suffix)
			}
		}
		used[name] = true
		params = append(params, name+": "+tsType(p.Type))
	}
	if imported {
		params = append(params, "options: { readonly signal: AbortSignal }")
	} else if c.Context {
		params = append(params, "options?: CallOptions")
	}
	result := "void"
	if c.Result != nil {
		result = tsType(c.Result)
		if c.NonNull {
			result = strings.TrimSuffix(result, " | null")
		}
	}
	if imported {
		result += " | Promise<" + result + ">"
	} else {
		result = "Promise<" + result + ">"
	}
	return c.Name + "(" + strings.Join(params, ", ") + "): " + result + ";"
}

func requiredImports(a *schema.API) bool {
	for _, d := range a.Declarations {
		if d.Kind == "imports" && !d.Optional {
			return true
		}
	}
	return false
}

func writeImports(b *bytes.Buffer, a *schema.API) {
	fmt.Fprintln(b, "export interface Imports {")
	for _, d := range a.Declarations {
		if d.Kind != "imports" {
			continue
		}
		writeDoc(b, "  ", declarationDoc(a, d.Doc))
		optional := ""
		if d.Optional {
			optional = "?"
		}
		if len(d.ImportMethods) == 0 {
			fmt.Fprintf(b, "  %s%s: object;\n", d.Name, optional)
			continue
		}
		fmt.Fprintf(b, "  %s%s: {\n", d.Name, optional)
		for _, c := range d.ImportMethods {
			writeDoc(b, "    ", declarationDoc(a, c.Doc))
			fmt.Fprintln(b, "    "+callSignature(c, true))
		}
		fmt.Fprintln(b, "  };")
	}
	fmt.Fprintln(b, "}")
}

func writeDeclaration(b *bytes.Buffer, a *schema.API, d *schema.Decl) {
	doc := declarationDoc(a, d.Doc)
	if d.Concurrent {
		if doc != "" {
			doc += "\n\n"
		}
		doc += "Ordinary methods may overlap. Stream and iterator operations retain their direction ordering."
	}
	writeDoc(b, "", doc)
	switch d.Kind {
	case "scalar":
		fmt.Fprintf(b, "export type %s = %s;\n", d.Name, tsType(d.Value))
		return
	case "enum":
		fmt.Fprintf(b, "export const %s: {\n", d.Name)
		for _, c := range d.Constants {
			writeDoc(b, "  ", declarationDoc(a, c.Doc))
			value := c.Value
			if d.Value.Kind == "int64" || d.Value.Kind == "uint64" {
				value += "n"
			}
			fmt.Fprintf(b, "  readonly %s: %s;\n", c.Name, value)
		}
		fmt.Fprintln(b, "};")
		if d.Validate == "enum" {
			fmt.Fprintf(b, "export type %s = (typeof %s)[keyof typeof %s];\n", d.Name, d.Name, d.Name)
		} else {
			fmt.Fprintf(b, "export type %s = %s;\n", d.Name, tsType(d.Value))
		}
		return
	case "error":
		fmt.Fprintf(b, "export interface %sData {\n", d.Name)
		for _, f := range d.Fields {
			writeDoc(b, "  ", declarationDoc(a, f.Doc))
			fmt.Fprintf(b, "  %s: %s;\n", f.Name, tsType(f.Type))
		}
		fmt.Fprintf(b, "}\n\nexport class %s extends Error {\n  readonly code: 'go-error';\n  readonly domain: %q;\n  readonly data: %sData;\n  constructor(message: string, data: %sData);\n}\n\nexport function is%s(value: unknown): value is %s;\n", d.Name, d.Domain, d.Name, d.Name, d.Name, d.Name)
		return
	}
	extends := ""
	if c := d.Methods["next"]; c != nil {
		extends = " extends AsyncIterableIterator<" + tsType(c.Result) + ">"
	}
	fmt.Fprintf(b, "export interface %s%s {\n", d.Name, extends)
	if d.Kind == "record" {
		for _, f := range d.Fields {
			writeDoc(b, "  ", declarationDoc(a, f.Doc))
			fmt.Fprintf(b, "  %s: %s;\n", f.Name, tsType(f.Type))
		}
	} else {
		fmt.Fprintf(b, "  readonly [resourceBrand]: %q;\n", d.Name)
		if d.Methods["read"] != nil {
			fmt.Fprintln(b, "  readonly readable: ReadableStream<Uint8Array>;")
		}
		if d.Methods["write"] != nil {
			fmt.Fprintln(b, "  readonly writable: WritableStream<Uint8Array>;")
		}
		for _, c := range a.Calls {
			if c.Receiver == d.Name && c.Op == "" {
				writeDoc(b, "  ", declarationDoc(a, c.Doc))
				fmt.Fprintln(b, "  "+callSignature(c, false))
			}
		}
		fmt.Fprintln(b, "  dispose(): Promise<void>;")
	}
	fmt.Fprintln(b, "}")
}

func writeOptions(b *bytes.Buffer, a *schema.API) {
	fmt.Fprintln(b, "export interface CallOptions {\n  signal?: AbortSignal;\n}\n\nexport interface LoadOptions {\n  startupTimeoutMs?: number;\n  maxPendingCalls?: number;\n  maxResources?: number;\n  maxValueBytes?: number;\n  maxValueDepth?: number;")
	if hasFeature(a, "streams") {
		fmt.Fprintln(b, "  streamChunkBytes?: number;\n  streamHighWaterBytes?: number;")
	}
	if hasFeature(a, "imports") {
		optional := "?"
		if requiredImports(a) {
			optional = ""
		}
		fmt.Fprintf(b, "  imports%s: Imports;\n  maxPendingImports?: number;\n", optional)
	}
	fmt.Fprintln(b, "}\n\nexport class WasmBindError extends Error {\n  readonly code: string;\n  constructor(code: string, message: string);\n}")
}

func declarations(a *schema.API) string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by wasmbind. DO NOT EDIT.")
	fmt.Fprint(&b, "declare const resourceBrand: unique symbol;\n\n")
	writeOptions(&b, a)
	if hasFeature(a, "imports") {
		fmt.Fprintln(&b)
		writeImports(&b, a)
	}
	for _, d := range a.Declarations {
		if d.Kind != "imports" {
			fmt.Fprintln(&b)
			writeDeclaration(&b, a, d)
		}
	}
	fmt.Fprintln(&b, "\nexport interface Module {")
	for _, c := range a.Calls {
		if c.Receiver == "" && c.Op == "" {
			writeDoc(&b, "  ", declarationDoc(a, c.Doc))
			fmt.Fprintln(&b, "  "+callSignature(c, false))
		}
	}
	optional := "?"
	if requiredImports(a) {
		optional = ""
	}
	fmt.Fprintf(&b, "  dispose(): Promise<void>;\n}\n\nexport function load(source: URL | Response | BufferSource | WebAssembly.Module, options%s: LoadOptions): Promise<Module>;\n", optional)
	return b.String()
}

// APIReference renders a reference from the same validated schema used for the
// runtime and TypeScript declarations.
func APIReference(a *schema.API) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<!-- %s -->\n\n# JavaScript API\n\nGo package: `%s`.\n\n", schema.Marker, a.Package)
	fmt.Fprintln(&b, "Call `load(source, options)` to create a module. Exported calls return promises; calls accepting `CallOptions` support an `AbortSignal`. Call `dispose()` on resources and the module when finished.")
	fmt.Fprintln(&b, "\n## Loading\n\n```ts")
	writeOptions(&b, a)
	optional := "?"
	if requiredImports(a) {
		optional = ""
	}
	fmt.Fprintf(&b, "\nexport function load(source: URL | Response | BufferSource | WebAssembly.Module, options%s: LoadOptions): Promise<Module>;\n```\n", optional)
	fmt.Fprintln(&b, "\n## Module functions")
	for _, c := range a.Calls {
		if c.Receiver == "" && c.Op == "" {
			fmt.Fprintf(&b, "\n### `%s`\n\n", c.Name)
			if c.Doc != "" {
				fmt.Fprintln(&b, declarationDoc(a, c.Doc)+"\n")
			}
			fmt.Fprintf(&b, "```ts\n%s\n```\n", callSignature(c, false))
		}
	}
	if hasFeature(a, "imports") {
		fmt.Fprintln(&b, "\n## Host imports\n\nRequired imports must be supplied in `LoadOptions.imports`. Optional import groups may be omitted to use the Go defaults. Each imported call receives an `AbortSignal` in its final options argument.\n\n```ts")
		writeImports(&b, a)
		fmt.Fprintln(&b, "```")
	}
	fmt.Fprintln(&b, "\n## Types")
	for _, d := range a.Declarations {
		if d.Kind != "imports" {
			fmt.Fprintf(&b, "\n### `%s`\n\n", d.Name)
			if d.Kind == "enum" {
				fmt.Fprintf(&b, "Constants are properties of `%s`, using the member names below. ", d.Name)
				if d.Validate == "enum" {
					fmt.Fprintln(&b, "Only the declared constant values are accepted.")
				} else {
					fmt.Fprintln(&b, "Other values of the underlying type are also accepted.")
				}
				fmt.Fprintln(&b)
			}
			fmt.Fprintln(&b, "```ts")
			writeDeclaration(&b, a, d)
			fmt.Fprintln(&b, "```")
		}
	}
	return b.Bytes()
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
