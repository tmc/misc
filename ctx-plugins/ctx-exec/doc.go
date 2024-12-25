/*
ctx-exec executes shell commands and wraps their output in XML-like tags or JSON format.

Usage: ctx-exec [flags] command

Flags:
  -color=true
    	Enable colored output (default: on for TTY)
  -escape=false
    	Enable escaping of special characters in output
  -exit-code=false
    	Use the exit code of the executed command
  -json=false
    	Output in JSON format instead of XML
  -shell=""
    	Specify the shell to use (default: bash or $SHELL)
  -tag=""
    	Override the output tag name (default: "exec-output")
  -x=false
    	Enable bash -x style tracing

Environment variables:
  CTX_EXEC_ESCAPE  Set to "true" to enable XML escaping
  CTX_EXEC_JSON    Set to "true" to enable JSON output
  CTX_EXEC_TAG     Override the default output tag name
  NO_COLOR         Disable colored output
  COLOR            Enable colored output

Examples:
	# Basic usage
	$ ctx-exec 'echo hello'
	<exec-output cmd="echo hello">
	<stdout>
	hello
	</stdout>
	</exec-output>

	# JSON output
	$ ctx-exec -json 'echo hello'
	{
	  "cmd": "echo hello",
	  "stdout": "hello\n"
	}

	# Custom tag name
	$ CTX_EXEC_TAG=custom ctx-exec 'echo hello'
	<custom cmd="echo hello">
	<stdout>
	hello
	</stdout>
	</custom>
*/
package main

// Usage is the usage message shown by flag.Usage.
const Usage = `Usage: ctx-exec [flags] command

Flags:
  -color=true
    	Enable colored output (default: on for TTY)
  -escape=false
    	Enable escaping of special characters in output
  -exit-code=false
    	Use the exit code of the executed command
  -json=false
    	Output in JSON format instead of XML
  -shell=""
    	Specify the shell to use (default: bash or $SHELL)
  -tag=""
    	Override the output tag name (default: "exec-output")
  -x=false
    	Enable bash -x style tracing

Environment variables:
  CTX_EXEC_ESCAPE  Set to "true" to enable XML escaping
  CTX_EXEC_JSON    Set to "true" to enable JSON output
  CTX_EXEC_TAG     Override the default output tag name
  NO_COLOR         Disable colored output
  COLOR            Enable colored output
`

