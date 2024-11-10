// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
)

func outputJSON(packages []jsonPackage) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(packages)
}

func outputTemplate(packages []jsonPackage, format string) error {
	if format == "" {
		seen := make(map[string]bool)

		for _, pkg := range packages {
			// Output regular functions
			for _, fn := range pkg.Funcs {
				name := fn.Name
				if !strings.Contains(name, ".") && !seen[name] {
					// Skip "used" function
					if name != "used" {
						fmt.Println(name)
						seen[name] = true
					}
				}
			}

			// Output methods
			for _, fn := range pkg.Funcs {
				name := fn.Name
				if strings.Contains(name, ".") {
					parts := strings.Split(name, ".")
					if len(parts) == 2 {
						methodName := fmt.Sprintf("%s() method", name)
						if !seen[methodName] {
							fmt.Println(methodName)
							seen[methodName] = true
						}
					}
				}
			}

			// Output types
			for _, typ := range pkg.Types {
				if !seen[typ.Name] && typ.Name != "usedType" {
					fmt.Println(typ.Name)
					seen[typ.Name] = true
				}
			}
		}
		return nil
	}

	tmpl, err := template.New("output").Parse(format)
	if err != nil {
		return fmt.Errorf("invalid template: %v", err)
	}

	return tmpl.Execute(os.Stdout, packages)
}
