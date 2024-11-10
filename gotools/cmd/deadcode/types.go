// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Output types for JSON and template formatting
type jsonPackage struct {
    Name      string          `json:"name"`
    Path      string          `json:"path"`
    Funcs     []jsonFunction  `json:"funcs,omitempty"`
    Types     []jsonType      `json:"types,omitempty"`
    Ifaces    []jsonInterface `json:"interfaces,omitempty"`
    Fields    []jsonField     `json:"fields,omitempty"`
}

type jsonFunction struct {
    Name      string       `json:"name"`
    Position  jsonPosition `json:"position"`
    Generated bool        `json:"generated,omitempty"`
}

type jsonType struct {
    Name     string       `json:"name"`
    Position jsonPosition `json:"position"`
}

type jsonInterface struct {
    Name     string       `json:"name"`
    Position jsonPosition `json:"position"`
}

type jsonField struct {
    Type     string       `json:"type"`
    Field    string       `json:"field"`
    Position jsonPosition `json:"position"`
}

type jsonPosition struct {
    File      string `json:"file"`
    Line, Col int    `json:"line,col"`
}
