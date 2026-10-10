package llmperf

import (
	"fmt"
	"testing"

	"go.k6.io/k6/v2/js/modulestest"
)

func TestTemplateJavaScriptContract(t *testing.T) {
	rt := modulestest.NewRuntime(t)
	if err := rt.SetupModuleSystem(map[string]any{importPath: new(rootModule)}, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, err := rt.RunOnEventLoop(fmt.Sprintf(`
 const {renderRequestTemplate} = require(%q);
 const result = renderRequestTemplate({name: 'openai-embeddings', variables: {model: 'fixture', texts: ['hello']}});
 if (result.body.input[0] !== 'hello' || result.provenance.schemaVersion !== 1 || result.provenance.templateSHA256.length !== 64) throw new Error('render contract');
 for (const spec of [{name:'openai-embeddings',variables:{}},{body:{a:{$var:'x'}},variables:{x:0},unknown:1}]) {
   let rejected=false;
   try { renderRequestTemplate(spec); } catch (e) { rejected=true; }
   if (!rejected) throw new Error('invalid template accepted');
 }
 `, importPath))
	if err != nil {
		t.Fatal(err)
	}
}
