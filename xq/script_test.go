package main

import (
    "bytes"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestXQ(t *testing.T) {
    tests, err := filepath.Glob("testdata/*.txt")
    if err != nil {
        t.Fatal(err)
    }

    for _, test := range tests {
        name := strings.TrimSuffix(filepath.Base(test), ".txt")
        t.Run(name, func(t *testing.T) {
            cmd := exec.Command("go", "run", "main.go")
            cmd.Stdin, err = os.Open(test)
            if err != nil {
                t.Fatal(err)
            }

            var stdout, stderr bytes.Buffer
            cmd.Stdout = &stdout
            cmd.Stderr = &stderr

            err = cmd.Run()
            if err != nil {
                t.Fatalf("command failed: %v\n%s", err, stderr.String())
            }

            got := stdout.String()
            want, err := os.ReadFile(test + ".out")
            if err != nil {
                t.Fatal(err)
            }

            if got != string(want) {
                t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
            }
        })
    }
}

func TestNewFeatures(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        input   string
        wantOut string
        wantErr string
    }{
        {
            name:    "HTML formatting",
            args:    []string{"-h"},
            input:   "<html><body><p>Hello</p></body></html>",
            wantOut: "<html>\n  <body>\n    <p>Hello</p>\n  </body>\n</html>\n",
        },
        {
            name:    "JSON output",
            args:    []string{"-j"},
            input:   "<root><child>value</child></root>",
            wantOut: "{\n  \"root\": {\n    \"child\": \"value\"\n  }\n}\n",
        },
        {
            name:    "Streaming large XML",
            args:    []string{"-S"},
            input:   "<root><child>value</child></root>",
            wantOut: "<root>\n  <child>value</child>\n</root>\n",
        },
        {
            name:    "Compact output",
            args:    []string{"-c"},
            input:   "<root><child>value</child></root>",
            wantOut: "<root><child>value</child></root>\n",
        },
        {
            name:    "XPath query",
            args:    []string{"-x", "//child"},
            input:   "<root><child>value</child></root>",
            wantOut: "<child>value</child>\n",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := exec.Command("go", append([]string{"run", "main.go"}, tt.args...)...)
            cmd.Stdin = strings.NewReader(tt.input)

            var stdout, stderr bytes.Buffer
            cmd.Stdout = &stdout
            cmd.Stderr = &stderr

            err := cmd.Run()
            if err != nil && tt.wantErr == "" {
                t.Fatalf("command failed: %v\n%s", err, stderr.String())
            }

            if got, want := stdout.String(), tt.wantOut; got != want {
                t.Errorf("output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
            }

            if got, want := stderr.String(), tt.wantErr; got != want {
                t.Errorf("error mismatch:\ngot:\n%s\nwant:\n%s", got, want)
            }
        })
    }
}

