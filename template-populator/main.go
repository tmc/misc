package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"regexp"
	"strings"
	texttemplate "text/template"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	templateFile string
	strictMode   bool
	envPriority  bool
	useHTML      bool
	jsonlMode    bool
	verbose      bool
}

func parseFlags(args []string) (*config, *flag.FlagSet, error) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	cfg := &config{}

	fs.StringVar(&cfg.templateFile, "template", "-", "Path to the template file (use '-' for stdin)")
	fs.BoolVar(&cfg.strictMode, "strict", false, "Exit with an error if any placeholder is not replaced")
	fs.BoolVar(&cfg.envPriority, "env-priority", false, "Give priority to environment variables over command-line arguments")
	fs.BoolVar(&cfg.useHTML, "html", false, "Use html/template instead of text/template")
	fs.BoolVar(&cfg.jsonlMode, "jsonl", false, "Process input as JSONL")
	fs.BoolVar(&cfg.verbose, "verbose", false, "Print verbose output")

	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}

	return cfg, fs, nil
}

func run(args []string) error {
	cfg, fs, err := parseFlags(args)
	if err != nil {
		return err
	}

	content, err := readTemplate(cfg.templateFile)
	if err != nil {
		return err
	}

	tmpl, err := parseTemplate(cfg.templateFile, content, cfg.useHTML, cfg.strictMode)
	if err != nil {
		return err
	}

	if cfg.jsonlMode {
		return processJSONL(tmpl, cfg, fs)
	}

	data := prepareData(cfg, fs)
	return tmpl.Execute(os.Stdout, data)
}

func readTemplate(templateFile string) (string, error) {
	var r io.Reader
	if templateFile == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(templateFile)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	}

	content, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return convertTemplateFormat(string(content)), nil
}

func convertTemplateFormat(content string) string {
	re := regexp.MustCompile(`{{([A-Za-z0-9_]+)}}`)
	return re.ReplaceAllString(content, "{{.$1}}")
}

func parseTemplate(fileName, content string, useHTML, strictMode bool) (interface {
	Execute(io.Writer, interface{}) error
}, error) {
	if useHTML {
		t := template.New(fileName)
		if strictMode {
			t = t.Option("missingkey=error")
		}
		return t.Parse(content)
	}

	t := texttemplate.New(fileName)
	if strictMode {
		t = t.Option("missingkey=error")
	}
	return t.Parse(content)
}

func prepareData(cfg *config, flagSet *flag.FlagSet) map[string]interface{} {
	data := make(map[string]interface{})

	if cfg.templateFile == "-" && !isTerminal(os.Stdin) {
		if stdin, err := io.ReadAll(os.Stdin); err == nil {
			if jsonData, err := parseJSON(string(stdin)); err == nil {
				for k, v := range jsonData {
					data[k] = v
				}
			}
		}
	}

	for _, env := range os.Environ() {
		if pair := strings.SplitN(env, "=", 2); len(pair) == 2 {
			if cfg.envPriority || data[pair[0]] == nil {
				data[pair[0]] = pair[1]
			}
		}
	}

	for _, arg := range flagSet.Args() {
		if strings.HasPrefix(arg, "{") && strings.HasSuffix(arg, "}") {
			if jsonData, err := parseJSON(arg); err == nil {
				for k, v := range jsonData {
					if !cfg.envPriority || data[k] == nil {
						data[k] = v
					}
				}
			}
		} else if pair := strings.SplitN(arg, "=", 2); len(pair) == 2 {
			if !cfg.envPriority || data[pair[0]] == nil {
				data[pair[0]] = pair[1]
			}
		}
	}

	return data
}

func processJSONL(tmpl interface {
	Execute(io.Writer, interface{}) error
}, cfg *config, flagSet *flag.FlagSet) error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		data, err := parseJSON(scanner.Text())
		if err != nil {
			return fmt.Errorf("parsing JSONL: %w", err)
		}
		for k, v := range prepareData(cfg, flagSet) {
			if cfg.envPriority || data[k] == nil {
				data[k] = v
			}
		}
		buf := new(strings.Builder)
		if err := tmpl.Execute(buf, data); err != nil {
			return fmt.Errorf("executing template: %w", err)
		}
		result := map[string]any{
			"output": buf.String(),
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("encoding result to JSON: %w", err)
		}
	}

	return scanner.Err()
}

type strictMap struct {
	m map[string]interface{}
}

func (s strictMap) Get(key string) (interface{}, error) {
	fmt.Fprintln(os.Stderr, "Key: ", key)

	if v, ok := s.m[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("missing value for key: %s", key)
}

func parseJSON(jsonString string) (map[string]interface{}, error) {
	var d map[string]interface{}
	err := json.Unmarshal([]byte(jsonString), &d)
	return d, err
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
