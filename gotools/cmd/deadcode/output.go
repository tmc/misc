// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "text/template"
)

func outputJSON(packages []jsonPackage) error {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(packages)
}

func outputTemplate(packages []jsonPackage, format string) error {
    tmpl, err := template.New("output").Parse(format)
    if err != nil {
        return fmt.Errorf("invalid template: %v", err)
    }
    for _, pkg := range packages {
        if err := tmpl.Execute(os.Stdout, pkg); err != nil {
            return err
        }
    }
    return nil
}
