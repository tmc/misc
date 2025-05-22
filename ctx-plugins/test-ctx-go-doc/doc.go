// Package main implements ctx-go-doc, a tool for fetching Go documentation in a structured format.
//
// ctx-go-doc is a wrapper around Go's standard `go doc` command that automatically handles
// package resolution and provides output suitable for LLM context.
//
// Features:
//   - Uses local package dependencies when available
//   - Automatically fetches external packages when needed
//   - Supports specific version requests with @version syntax
//   - Caches fetched packages for better performance
//   - Formats output in an XML-like structured format
//
// Usage:
//
//	ctx-go-doc [options] package [name ...]
//
// Examples:
//
//	# Get package documentation
//	ctx-go-doc fmt
//
//	# Get specific function documentation
//	ctx-go-doc fmt Println
//
//	# Get documentation for a specific version of a package
//	ctx-go-doc github.com/pkg/errors@v0.9.1
//
// Environment Variables:
//   - CTX_GO_DOC_DEBUG=true: Enable debug output
//
// Output Format:
//   The output is wrapped in XML-like tags for easy parsing:
//
//	<exec-output cmd="go doc package ...">
//	  <stdout>
//	    ... documentation output ...
//	  </stdout>
//	  <stderr>
//	    ... error output (if any) ...
//	  </stderr>
//	  <e>
//	    ... error message (if any) ...
//	  </e>
//	</exec-output>
//
package main