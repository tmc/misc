package main

import (
	"fmt"
	"net/http"
	"os"
)

var ErrNotFound = fmt.Errorf("not found")

// curl -H'Accept: application/vnd.docker.distribution.manifest.v2+json' https://registry.ollama.ai/v2/library/fixzbuz/manifests/latest
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: %s <model> [<tag>]")
		os.Exit(1)
	}
	model := os.Args[1]
	tag := "latest"
	if len(os.Args) > 2 {
		tag = os.Args[2]
	}
	if err := run(model, tag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(model, tag string) error {
	url := fmt.Sprintf("https://registry.ollama.ai/v2/library/%s/manifests/%s", model, tag)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	r.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	return nil
}
