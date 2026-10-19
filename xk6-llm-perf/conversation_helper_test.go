package llmperf

import (
	"os"
	"strings"
	"testing"

	"github.com/grafana/sobek"
)

func TestConversationHelper(t *testing.T) {
	source, err := os.ReadFile("examples/lib/workload.js")
	if err != nil {
		t.Fatal(err)
	}
	// Execute the helper without network or timers. Other helper declarations
	// remain present, so this exercises the shipped source directly.
	text := strings.ReplaceAll(string(source), "import { sleep } from 'k6';", "function sleep() {}")
	text = strings.ReplaceAll(text, "import crypto from 'k6/crypto';", "")
	text = strings.ReplaceAll(text, "export function", "function")
	text = strings.ReplaceAll(text, "export async function", "async function")
	rt := sobek.New()
	if _, err := rt.RunString(text); err != nil {
		t.Fatal(err)
	}
	_, err = rt.RunString(`
const calls = [];
const client = {chat:{completions:{create(request) {
    calls.push(JSON.parse(JSON.stringify(request)));
    return {choices:[{message:{role:'assistant',content:'answer'}}]};
}}}};
const work = {minOutputTokens:2};
const initial = [{role:'system',content:'system'}];
const turns = [{prompt:'first',request:{max_tokens:8,temperature:0,work,extensions:{ignore_eos:true}}}, {prompt:'second',request:{max_tokens:16,extensions:undefined}}];
conversation(client, turns, initial);
if (calls.length !== 2 || calls[0].max_tokens !== 8 || calls[0].temperature !== 0 || calls[0].work.minOutputTokens !== 2 || calls[1].max_tokens !== 16) throw new Error('request options lost');
if (calls[0].extensions.ignore_eos !== true || calls[1].extensions !== undefined) throw new Error('request extensions lost');
if (calls[0].messages.length !== 2 || calls[1].messages.length !== 4 || !calls.every(r => r.stream)) throw new Error('conversation context lost');
if (initial.length !== 1 || turns[0].request.messages !== undefined) throw new Error('input mutated');
calls.length = 0;
conversation(client, [{prompt:'first'}, {prompt:'second'}], initial, 'independent');
if (calls.some(r => r.messages.length !== 2)) throw new Error('independent context lost');
for (const request of [null, [], 'options', {messages:[]}, {stream:false}, {extensions:null}, {extensions:[]}, {extensions:{messages:[]}}, {extensions:{Stream:false}}, {Messages:[]}, {'\u017ftream':false}, {Extensions:{stream:false}}, {Extensions:{'\u017ftream':false}}, {extensions:{},Extensions:{}}]) {
    calls.length = 0;
    let rejected = false;
    try { conversation(client, [{prompt:'valid'}, {prompt:'invalid',request}]); } catch (error) { rejected = true; }
    if (!rejected || calls.length) throw new Error('invalid options caused partial dispatch');
}
calls.length = 0;
const failure = new Error('fixture');
client.chat.completions.create = request => { calls.push(request); return failure; };
const results = conversation(client, [{prompt:'first'}, {prompt:'second'}]);
if (calls.length !== 1 || results.length !== 1 || results[0] !== failure) throw new Error('failure did not stop conversation');
`)
	if err != nil {
		t.Fatal(err)
	}
}
