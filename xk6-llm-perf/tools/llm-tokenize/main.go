// Command llm-tokenize counts text with pinned offline BPE and role-frame assets.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"unicode/utf8"

	"github.com/tmc/lib/hf/hftokenizer"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type input struct {
	Text                *string   `json:"text"`
	Messages            []message `json:"messages"`
	AddGenerationPrompt bool      `json:"add_generation_prompt"`
}
type frame struct {
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}
type framing struct {
	SchemaVersion    int              `json:"schema_version"`
	Prefix           string           `json:"prefix"`
	Suffix           string           `json:"suffix"`
	Roles            map[string]frame `json:"roles"`
	GenerationPrompt string           `json:"generation_prompt"`
}
type result struct {
	SchemaVersion   int     `json:"schema_version"`
	Model           string  `json:"model"`
	ModelRevision   string  `json:"model_revision"`
	TokenizerModule string  `json:"tokenizer_module"`
	TokenizerHash   string  `json:"tokenizer_sha256"`
	TemplateHash    string  `json:"template_sha256,omitempty"`
	TemplateDialect string  `json:"template_dialect,omitempty"`
	Scope           string  `json:"scope"`
	FramedHash      string  `json:"framed_sha256"`
	Count           int     `json:"count"`
	IDs             []int32 `json:"ids,omitempty"`
	Measurement     struct {
		InputTokens int    `json:"inputTokens"`
		Tokenizer   string `json:"tokenizer"`
	} `json:"measurement"`
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-tokenize:", err)
		os.Exit(1)
	}
}

func run(args []string, in io.Reader, out io.Writer) error {
	flags := flag.NewFlagSet("llm-tokenize", flag.ContinueOnError)
	inputPath := flags.String("input", "-", "input JSON file, or - for stdin")
	vocab := flags.String("tokenizer", "", "local tokenizer.json")
	vocabHash := flags.String("tokenizer-sha256", "", "required tokenizer SHA-256")
	template := flags.String("template", "", "optional local role-frame JSON")
	templateHash := flags.String("template-sha256", "", "required template SHA-256 when framing chat")
	model := flags.String("model", "", "declared model identity")
	revision := flags.String("revision", "", "declared model revision")
	idsFlag := flags.Bool("ids", false, "include token IDs (at most 100000)")
	post := flags.Bool("postprocessor", false, "apply tokenizer single-input postprocessor for text")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *vocab == "" || *model == "" || *revision == "" || len(*model) > 1024 || len(*revision) > 256 {
		return fmt.Errorf("tokenizer, model and revision are required")
	}
	data, err := asset(*vocab, *vocabHash, 64*1024*1024)
	if err != nil {
		return fmt.Errorf("vocabulary: %w", err)
	}
	tokenizer, err := hftokenizer.New(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if *inputPath != "-" {
		f, err := os.Open(*inputPath)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	var request input
	if err := decode(in, 4*1024*1024, &request); err != nil {
		return fmt.Errorf("input: %w", err)
	}
	if (request.Text != nil) == (request.Messages != nil) {
		return fmt.Errorf("provide exactly one of text or messages")
	}
	r := result{SchemaVersion: 1, Model: *model, ModelRevision: *revision, TokenizerModule: moduleVersion(), TokenizerHash: *vocabHash, Scope: "text without postprocessor"}
	text := ""
	if request.Text != nil {
		if *template != "" || *templateHash != "" || request.AddGenerationPrompt {
			return fmt.Errorf("chat framing options require messages")
		}
		text = *request.Text
		if *post {
			r.Scope = "text with asset-defined single-input postprocessor"
		}
	} else {
		if *template == "" || *templateHash == "" || *post {
			return fmt.Errorf("chat requires pinned framing; postprocessor must be disabled")
		}
		data, err := asset(*template, *templateHash, 1024*1024)
		if err != nil {
			return fmt.Errorf("template: %w", err)
		}
		var f framing
		if err := decode(bytes.NewReader(data), 1024*1024, &f); err != nil {
			return fmt.Errorf("template: %w", err)
		}
		text, err = render(request, f)
		if err != nil {
			return err
		}
		r.TemplateHash = *templateHash
		r.TemplateDialect = "llm-role-frame-v1"
		r.Scope = "rendered chat without tokenizer postprocessor"
	}
	var ids []int32
	if *post {
		ids, err = tokenizer.Encode(text)
	} else {
		ids, err = tokenizer.EncodeRaw(text)
	}
	if err != nil {
		return err
	}
	r.Count = len(ids)
	if *idsFlag {
		if len(ids) > 100000 {
			return fmt.Errorf("token ID export exceeds 100000")
		}
		r.IDs = ids
	}
	hash := sha256.Sum256([]byte(text))
	r.FramedHash = hex.EncodeToString(hash[:])
	r.Measurement.InputTokens = r.Count
	r.Measurement.Tokenizer = fmt.Sprintf("offline-bpe/vocab:%s/frame:%s/scope:%s", *vocabHash, *templateHash, r.Scope)
	return json.NewEncoder(out).Encode(r)
}

func asset(path, expected string, limit int64) ([]byte, error) {
	pin, err := hex.DecodeString(expected)
	if err != nil || len(pin) != sha256.Size {
		return nil, fmt.Errorf("asset SHA-256 pin is required")
	}
	// #nosec G304 -- Caller-selected offline asset is bounded and checked against its SHA-256 pin.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("asset exceeds byte budget")
	}
	got := sha256.Sum256(data)
	if !bytes.Equal(got[:], pin) {
		return nil, fmt.Errorf("asset SHA-256 mismatch")
	}
	return data, nil
}

func decode(r io.Reader, limit int64, v any) error {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("JSON exceeds byte budget")
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func render(request input, f framing) (string, error) {
	if f.SchemaVersion != 1 || len(request.Messages) == 0 || len(request.Messages) > 1000 || len(f.Roles) == 0 || len(f.Roles) > 16 {
		return "", fmt.Errorf("invalid role framing schema or message budget")
	}
	var b bytes.Buffer
	write := func(s string) error {
		if b.Len()+len(s) > 4*1024*1024 {
			return fmt.Errorf("framed input exceeds 4 MiB")
		}
		_, err := b.WriteString(s)
		return err
	}
	if err := write(f.Prefix); err != nil {
		return "", err
	}
	for _, m := range request.Messages {
		role, ok := f.Roles[m.Role]
		if !ok {
			return "", fmt.Errorf("role %q absent from framing asset", m.Role)
		}
		for _, s := range []string{role.Prefix, m.Content, role.Suffix} {
			if err := write(s); err != nil {
				return "", err
			}
		}
	}
	if err := write(f.Suffix); err != nil {
		return "", err
	}
	if request.AddGenerationPrompt {
		if f.GenerationPrompt == "" {
			return "", fmt.Errorf("generation prompt absent from framing asset")
		}
		if err := write(f.GenerationPrompt); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func moduleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/tmc/lib/hf/hftokenizer" {
				return d.Path + "@" + d.Version
			}
		}
	}
	return "github.com/tmc/lib/hf/hftokenizer@unknown"
}
