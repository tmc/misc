package generate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

func (e *emitter) extensionCall(b *bytes.Buffer, c *schema.Call) bool {
	switch c.Op {
	case "read":
		fmt.Fprintln(b, `if a0<=0 || a0>b.limits.MaxValueBytes{return js.Undefined(),failure("limit","invalid read chunk size")};p:=make([]byte,a0);n,err:=recv.`+c.GoName+`(i.ctx,p)
if n<0 || n>len(p){return js.Undefined(),failure("protocol","invalid read count")}
eof:=errors.Is(err,io.EOF);if err!=nil {r.finished["read"]=true};message:="";var domain any;if err!=nil && !eof {message=err.Error();domain=b.domainError(err)};out:=js.Global().Get("Uint8Array").New(n);js.CopyBytesToJS(out,p[:n]);return js.ValueOf(map[string]any{"data":out,"eof":eof,"message":message,"domain":domain}),nil`)
	case "write":
		fmt.Fprintln(b, `for len(a0)>0 {n,err:=recv.`+c.GoName+`(i.ctx,a0);if n<0 || n>len(a0){return js.Undefined(),failure("protocol","invalid write count")};if err!=nil{return js.Undefined(),err};if n==0{return js.Undefined(),io.ErrShortWrite};a0=a0[n:]};return js.Undefined(),nil`)
	case "finish":
		fmt.Fprintln(b, `err:=recv.`+c.GoName+`(i.ctx);if err==nil {r.finished["write"]=true};return js.Undefined(),err`)
	case "cancelread", "cancelwrite", "interrupt":
		lane := map[string]string{"cancelread": "read", "cancelwrite": "write", "interrupt": "event"}[c.Op]
		fmt.Fprintf(b, "b.startHook(r,%q);if h:=r.hooks[%q];h!=nil{<-h.done;return js.Undefined(),h.err};return js.Undefined(),nil\n", lane, lane)
	case "next":
		n := e.converter(c.Result)
		fmt.Fprintf(b, "value,more,err:=recv.%s(i.ctx);if err!=nil{return js.Undefined(),err};if !more{r.finished[\"event\"]=true;return js.ValueOf(map[string]any{\"done\":true}),nil};wire,err:=b.encode%d(value,i,&valueBudget{limits:b.limits},0);if err!=nil{return js.Undefined(),err};return js.ValueOf(map[string]any{\"done\":false,\"value\":wire}),nil\n", c.GoName, n)
	default:
		return false
	}
	return true
}
func (e *emitter) hookMap(d *schema.Decl) string {
	var parts []string
	for _, pair := range [][2]string{{"cancelread", "read"}, {"cancelwrite", "write"}, {"interrupt", "event"}} {
		if c := d.Methods[pair[0]]; c != nil {
			parts = append(parts, fmt.Sprintf("%q: {fn: func()error{return v.%s()},done:make(chan struct{})}", pair[1], c.GoName))
		}
	}
	return "map[string]*interruptHook{" + strings.Join(parts, ",") + "}"
}
func (e *emitter) extensions() string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "func (b *runtimeBridge) initialize()error {")
	if e.api.Init == nil {
		fmt.Fprintln(&b, "return nil")
	} else {
		var args []string
		args = append(args, "b.ctx")
		fmt.Fprintln(&b, `provided:=map[string]bool{};if err:=json.Unmarshal([]byte(os.Getenv("WASMBIND_IMPORTS")),&provided);err!=nil{return failure("argument","invalid import configuration")}`)
		for _, p := range e.api.Init.Params {
			d := e.decl(p.Type.Name)
			if d.Optional {
				fmt.Fprintf(&b, "var import%s api.%s;if provided[%q] {import%s=&host%s{b:b}} else {import%s=api.%s();if import%s==nil{return failure(\"go-error\",\"default import factory returned nil\")}}\n", d.Name, d.GoName, d.Name, d.Name, d.Name, d.Name, d.Default, d.Name)
				args = append(args, "import"+d.Name)
			} else {
				args = append(args, "&host"+p.Type.Name+"{b:b}")
			}
		}
		fmt.Fprintf(&b, "return api.%s(%s)\n", e.api.Init.GoName, strings.Join(args, ","))
	}
	fmt.Fprintln(&b, "}")
	for _, d := range e.api.Declarations {
		if d.Kind != "imports" {
			continue
		}
		fmt.Fprintf(&b, "type host%s struct{b *runtimeBridge}\n", d.Name)
		for _, c := range d.ImportMethods {
			var params, args []string
			params = append(params, "ctx context.Context")
			for j, p := range c.Params {
				params = append(params, fmt.Sprintf("a%d %s", j, p.Type.Go))
				args = append(args, fmt.Sprintf("a%d", j))
			}
			result := "error"
			if c.Result != nil {
				result = "(" + c.Result.Go + ",error)"
			}
			fmt.Fprintf(&b, "func (h *host%s) %s(%s)%s {\n", d.Name, c.GoName, strings.Join(params, ","), result)
			bad := "return err"
			if c.Result != nil {
				fmt.Fprintf(&b, "var zero %s\n", c.Result.Go)
				bad = "return zero,err"
			}
			fmt.Fprintln(&b, "q:=&valueBudget{limits:h.b.limits};_ = q;i:=&invocation{resources:map[int]*resource{},created:map[int]*resource{}};args:=js.Global().Get(\"Array\").New("+fmt.Sprint(len(args))+")")
			for j, p := range c.Params {
				n := e.converter(p.Type)
				fmt.Fprintf(&b, "w%d,err:=h.b.encode%d(a%d,i,q,0);if err!=nil{%s};args.SetIndex(%d,w%d)\n", j, n, j, bad, j, j)
			}
			fmt.Fprintf(&b, "wire,err:=h.b.importCall(ctx,%q,%q,args);if err!=nil{%s}\n", d.Name, c.Name, bad)
			if c.Result == nil {
				fmt.Fprintln(&b, "_ = wire;return nil")
			} else {
				n := e.converter(c.Result)
				fmt.Fprintf(&b, "v,err:=h.b.decode%d(wire,i,&valueBudget{limits:h.b.limits},0);if err!=nil{return zero,fmt.Errorf(\"import %s.%s: %%w\",err)};return v,nil\n", n, d.Name, c.Name)
			}
			fmt.Fprintln(&b, "}")
		}
	}
	fmt.Fprintln(&b, "func (b *runtimeBridge) domainError(err error)any {")
	fmt.Fprintln(&b, `message:=err.Error();_ = message;var walk func(error,int)any;nodes:=0;var path []error
walk=func(current error,depth int)any{if current==nil || depth>64 || nodes>=256{return nil};nodes++;for _,p:=range path{if reflect.TypeOf(p)==reflect.TypeOf(current)&&reflect.TypeOf(p).Comparable()&&p==current{return nil}};path=append(path,current);defer func(){path=path[:len(path)-1]}()`)
	for _, d := range e.api.Declarations {
		if d.Kind != "error" {
			continue
		}
		n := e.converter(&schema.Type{Kind: "record", Name: d.Name, Go: "api." + d.GoName})
		fmt.Fprintf(&b, "if value,ok:=current.(*api.%s);ok && value!=nil {wire,convertErr:=b.encode%d(*value,&invocation{resources:map[int]*resource{},created:map[int]*resource{}},&valueBudget{limits:b.limits},0);if convertErr!=nil{return map[string]any{\"code\":\"go-error\",\"message\":message}};return map[string]any{\"code\":\"go-error\",\"message\":message,\"domain\":%q,\"data\":wire}}\n", d.GoName, n, d.Domain)
	}
	fmt.Fprintln(&b, `if e,ok:=current.(interface{Unwrap()[]error});ok {for _,child:=range e.Unwrap(){if found:=walk(child,depth+1);found!=nil{return found}}} else if e,ok:=current.(interface{Unwrap()error});ok {return walk(e.Unwrap(),depth+1)};return nil};if found:=walk(err,0);found!=nil{return found};return nil
}`)
	return b.String()
}

func domainClasses(a *schema.API) string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "const domainClasses=new Map(),domainBrands=new WeakMap();")
	for _, d := range a.Declarations {
		if d.Kind != "error" {
			continue
		}
		fmt.Fprintf(&b, `export class %s extends Error {
constructor(message,data){super(message);this.name=%q;Object.defineProperties(this,{code:{value:'go-error',enumerable:true},domain:{value:%q,enumerable:true},data:{value:data,enumerable:true}});domainBrands.set(this,%q);}
}
export function is%s(value){return domainBrands.get(value)===%q;}
domainClasses.set(%q,{name:%q,type:%s});
`, d.Name, d.Name, d.Domain, d.Domain, d.Name, d.Domain, d.Domain, d.Name, d.Name)
	}
	return b.String()
}
