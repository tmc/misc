package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func pin(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
func tokenizerArgs(t *testing.T) []string {
	return []string{"-tokenizer", "testdata/tokenizer.json", "-tokenizer-sha256", pin(t, "testdata/tokenizer.json"), "-model", "synthetic-gpt2", "-revision", "fixture@7e9483795cd2", "-ids"}
}

func TestPinnedTokensAndFraming(t *testing.T) {
	args := tokenizerArgs(t)
	var out bytes.Buffer
	if err := run(args, strings.NewReader(`{"text":"hello world"}`), &out); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []int32{168, 205, 204, 204, 167, 32, 239, 167, 130, 204, 132}
	if !reflect.DeepEqual(got.IDs, want) || got.Count != 11 {
		t.Fatalf("tokens=%+v", got)
	}
	args = append(args, "-template", "testdata/framing.json", "-template-sha256", pin(t, "testdata/framing.json"))
	out.Reset()
	if err := run(args, strings.NewReader(`{"messages":[{"role":"user","content":"hello world"}],"add_generation_prompt":true}`), &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want = append([]int32{264}, want...)
	want = append(want, 265, 266)
	if !reflect.DeepEqual(got.IDs, want) || got.Count != 14 || got.Measurement.InputTokens != 14 || got.TemplateDialect != "llm-role-frame-v1" {
		t.Fatalf("chat=%+v", got)
	}
}
func TestAssetAndScopeRefusals(t *testing.T) {
	args := tokenizerArgs(t)
	for _, input := range []string{`{"text":"a","messages":[]}`, `{"messages":[]}`, `{"text":"a","unknown":1}`, `{"text":"a"}{}`, `{"messages":[{"role":"missing","content":"a"}]}`} {
		if err := run(args, strings.NewReader(input), &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	args[3] = strings.Repeat("0", 64)
	if err := run(args, strings.NewReader(`{"text":"a"}`), &bytes.Buffer{}); err == nil {
		t.Fatal("accepted hash mismatch")
	}
}
func TestCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "llm-tokenize")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	vocab, err := filepath.Abs("testdata/tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := filepath.Abs("testdata/framing.json")
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "CLI="+binary, "VOCAB="+vocab, "VOCAB_HASH="+pin(t, vocab), "FRAME="+frame, "FRAME_HASH="+pin(t, frame))
	engine := &script.Engine{Cmds: scripttest.DefaultCmds(), Conds: scripttest.DefaultConds()}
	scripttest.Test(t, ctx, engine, env, "testdata/*.txt")
}

func TestKnownUnicodeAndSpecialTokenSequences(t *testing.T) {
	data, err := os.ReadFile("testdata/known-tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text string  `json:"text"`
		IDs  []int32 `json:"ids"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	args := tokenizerArgs(t)
	for _, c := range cases {
		input, err := json.Marshal(map[string]string{"text": c.Text})
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := run(args, bytes.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		var got result
		if err := json.Unmarshal(output.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.IDs, c.IDs) && !(len(got.IDs) == 0 && len(c.IDs) == 0) {
			t.Fatalf("%q: ids=%v want=%v", c.Text, got.IDs, c.IDs)
		}
	}
}
