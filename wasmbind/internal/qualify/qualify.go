// Package qualify checks a local npm package with explicit consumer tools.
package qualify

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed runner.mjs
var runner []byte

// Options selects local tools and consumer fixtures. Every required gate runs;
// missing compilers or browser executables are errors, never skipped checks.
type Options struct {
	Package    string   `json:"package"`
	Out        string   `json:"out"`
	TSC        string   `json:"tsc"`
	NPM        string   `json:"npm"`
	Node       string   `json:"node"`
	Playwright string   `json:"playwright"`
	TimeoutMS  int      `json:"timeoutMs"`
	Fixtures   string   `json:"fixtures,omitempty"`
	Consumer   string   `json:"consumer"`
	Examples   []string `json:"examples"`
	Hook       string   `json:"hook,omitempty"`
}

// Run installs a local npm archive offline, checks a consumer and documentation
// examples in DOM and Worker environments, and observes load/dispose in all
// three Playwright browsers in Window and Worker. It writes an evidence receipt
// only after every required check succeeds. Out must not already exist.
func Run(ctx context.Context, o Options) error {
	if o.Package == "" || o.Out == "" || o.TSC == "" || o.NPM == "" || o.Node == "" || o.Playwright == "" || o.Consumer == "" || len(o.Examples) == 0 {
		return fmt.Errorf("qualify requires -package -out -tsc -npm -node -playwright -consumer and at least one example file")
	}
	if o.TimeoutMS == 0 {
		o.TimeoutMS = 30000
	}
	if o.TimeoutMS < 1 || o.TimeoutMS > 300000 {
		return fmt.Errorf("qualify timeout must be between 1 and 300000 milliseconds")
	}
	paths := []*string{&o.Package, &o.Out, &o.Playwright, &o.Consumer}
	if o.Fixtures != "" {
		paths = append(paths, &o.Fixtures)
	}
	if o.Hook != "" {
		paths = append(paths, &o.Hook)
	}
	for _, p := range paths {
		v, err := filepath.Abs(*p)
		if err != nil {
			return err
		}
		*p = v
	}
	for i, p := range o.Examples {
		v, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		o.Examples[i] = v
	}
	for _, p := range []*string{&o.TSC, &o.NPM, &o.Node} {
		v, err := exec.LookPath(*p)
		if err != nil {
			return fmt.Errorf("resolve tool: %w", err)
		}
		v, err = filepath.Abs(v)
		if err != nil {
			return err
		}
		*p = v
	}
	if err := os.Mkdir(o.Out, 0755); err != nil {
		return fmt.Errorf("create qualification directory: %w", err)
	}
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}
	config := filepath.Join(o.Out, "options.json")
	if err = os.WriteFile(config, data, 0644); err != nil {
		return err
	}
	script := filepath.Join(o.Out, "runner.mjs")
	if err = os.WriteFile(script, runner, 0644); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, o.Node, script, config)
	output, err := cmd.CombinedOutput()
	if logErr := os.WriteFile(filepath.Join(o.Out, "qualification.log"), output, 0644); logErr != nil {
		return logErr
	}
	if err != nil {
		return fmt.Errorf("qualify: %w\n%s", err, output)
	}
	return nil
}
