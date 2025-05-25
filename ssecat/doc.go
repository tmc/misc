// Command ssecat processes Server-Sent Events (SSE) streams and extracts data based on JSON paths.
//
// This tool reads SSE streams from files or stdin, parses JSON data within events,
// and extracts values based on specified JSON paths. It's particularly useful for
// processing streaming API responses from services like OpenAI, Anthropic, and other
// LLM providers. The tool also supports structured output of content blocks including
// text and tool use blocks from Claude API responses.
//
// Usage:
//
//	ssecat [flags] [files...]
//
// Flags:
//
//	-f file      Input file (default: "-" for stdin)
//	-delay dur   Delay between processing chunks
//	-path paths  Space-separated JSON path patterns to extract
//	-w           Run HTTP server that streams output
//	-port n      HTTP server port when using -w (default: 8072)
//	-m           Include metadata (tokens, stop reason) in output
//	-v           Verbose output (print formatted JSON)
//	-vv          Very verbose output (print raw lines and formatted JSON)
//	-fail-on-error  Exit with error on JSON parsing failures (default: true)
//	-cat-non-sse Output raw content for non-SSE input instead of error (default: true)
//	-progress    Show content block progress on stderr during streaming
//	-omit-message-start  Omit message_start events from output
//
// Examples:
//
//	# Process SSE stream from stdin
//	curl -N https://api.example.com/stream | ssecat
//
//	# Extract specific JSON paths from Claude API responses
//	ssecat -path "type=content_block_delta,delta.text" claude-response.txt
//
//	# Run as HTTP server streaming responses
//	ssecat -w -port 8080 &
//	curl http://localhost:8080/
//
//	# Process with metadata output
//	ssecat -m stream.txt
//
//	# Show streaming progress on stderr
//	ssecat -progress stream.txt
//
//	# Process without message_start events
//	ssecat -omit-message-start stream.txt
//
// The tool supports complex JSON path patterns for extracting nested data
// from streaming responses. Multiple paths can be specified and will be
// processed in order. When processing Claude API responses, the tool automatically
// handles content blocks, outputting text blocks as plain text and tool use blocks
// as JSON.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md
