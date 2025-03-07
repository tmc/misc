// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"reflect"
)

// debugPrintMap prints the contents of a map to stderr for debugging.
// Only prints if debug flag is true.
func debugPrintMap(name string, m interface{}) {
	if !*debugFlag {
		return
	}
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Map {
		fmt.Fprintf(os.Stderr, "DEBUG: %s is not a map\n", name)
		return
	}
	
	fmt.Fprintf(os.Stderr, "DEBUG: %s contains %d entries\n", name, v.Len())
	
	keys := v.MapKeys()
	for _, key := range keys {
		fmt.Fprintf(os.Stderr, "DEBUG: %s[%v] = %v\n", name, key, v.MapIndex(key))
	}
}