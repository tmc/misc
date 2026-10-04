package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAsyncDisposeFallback(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is unavailable")
	}
	raw, err := templates.ReadFile("templates/bindings.js")
	if err != nil {
		t.Fatal(err)
	}
	extensions, err := templates.ReadFile("templates/extensions.js")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.NewReplacer("__SCHEMA__", `{"declarations":[{"kind":"resource","name":"Thing"}],"calls":[{"id":0,"name":"create","params":[],"result":{"kind":"resource","name":"Thing"}}]}`, "__ARTIFACT_ID__", `"test"`, "__DOMAINS__", "const domainClasses=new Map();", "__EXTENSIONS__", string(extensions), "export ", "").Replace(string(raw))
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.js")
	if err = os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	runner := `const fs=require('node:fs'),vm=require('node:vm');const original=Symbol.asyncDispose;const symbol=Object.create(Symbol);Object.defineProperty(symbol,'asyncDispose',{value:undefined});const context={Symbol:symbol,TextEncoder,DOMException,queueMicrotask,console};vm.createContext(context);vm.runInContext(fs.readFileSync(process.argv[1],'utf8')+`
	runner += "`" + `
 let live,closed=0;
 live=makeModule({dispatch(id){queueMicrotask(()=>live.complete(id,'','',1));},disposeResource(id){closed++;queueMicrotask(()=>live.complete(id,'','',null));},shutdown(id){queueMicrotask(()=>live.complete(id,'','',null));}}, {maxPendingCalls:10,maxResources:10,maxValueBytes:100,maxValueDepth:10,maxElements:100},new Map());
 globalThis.result=(async()=>{const resource=await live.module.create();if(resource[asyncDispose]!==resource.dispose)throw Error('resource symbol missing');await resource[asyncDispose]();await live.module[asyncDispose]();if(closed!==1 || Symbol.asyncDispose!==undefined)throw Error('fallback changed Symbol');})();
 ` + "`" + `,context);Promise.resolve(context.result).then(()=>{if(Symbol.asyncDispose!==original)throw Error('global Symbol changed')}).catch(e=>{console.error(e);process.exitCode=1});`
	command := exec.Command("node", "-e", runner, path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("disposal fallback: %v\n%s", err, output)
	}
}
