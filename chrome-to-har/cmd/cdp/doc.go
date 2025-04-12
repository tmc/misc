/*
cdp: Interactive Chrome DevTools Protocol Command-Line Tool

cdp launches a Google Chrome instance and provides an interactive command-line
prompt (REPL) for executing raw Chrome DevTools Protocol (CDP) commands.
This allows direct interaction with the browser's debugging and introspection APIs,
facilitating tasks like programmatic debugging, automation, and exploring protocol
capabilities.

Usage:

	cdp [flags]

Flags:

	-headless
	    Run Chrome in headless mode (without UI). Recommended for scripting
	    or CI environments. Automatically enables --disable-gpu.
	-profile <name>
	    Load Chrome with the specified user profile directory name
	    (e.g., "Default", "Profile 1"). This allows using existing
	    extensions, cookies, and settings. If the profile is not found,
	    a warning is logged, and Chrome launches without a specific profile.
	-url <url>
	    Navigate to the specified URL upon starting Chrome.
	    (default: "about:blank")
	-chrome-path <path>
	    Explicit path to the Chrome executable. If empty, cdp attempts
	    to find Chrome automatically.
	-debug-port <port>
	    Connect to an already running Chrome instance listening on the
	    specified debugging port. If 0 (default), cdp launches a new
	    Chrome instance on a random debugging port.
	-timeout <seconds>
	    Maximum time in seconds to wait for Chrome to launch and for
	    individual CDP commands to complete. (default: 60)
	-verbose
	    Enable verbose logging, including browser console output and
	    internal cdp tool messages.

Interactive Prompt:

Once connected, cdp presents a "cdp> " prompt. You can enter CDP commands
or use predefined aliases.

Command Format:

	Domain.method {JSON parameters}

Examples:

	# Evaluate JavaScript
	cdp> Runtime.evaluate {"expression": "navigator.userAgent"}

	# Navigate the current page
	cdp> Page.navigate {"url": "https://google.com"}

	# Enable the debugger (also enabled by default)
	cdp> Debugger.enable {}

	# Set a breakpoint
	cdp> Debugger.setBreakpoint {"location": {"scriptId": "123", "lineNumber": 42}}

Aliases:

cdp provides aliases for common debugging and coverage tasks. Type 'help aliases'
in the prompt for a full list.

	# Debugger Aliases
	pause         # Alias for Debugger.pause {}
	resume | cont # Alias for Debugger.resume {}
	step | stepinto # Alias for Debugger.stepInto {}
	next | stepover # Alias for Debugger.stepOver {}
	out | stepout  # Alias for Debugger.stepOut {}

	# JS Coverage Aliases (Profiler Domain)
	covjs_enable  # Alias for Profiler.enable {}
	covjs_start   # Start precise JS coverage collection
	covjs_take    # Take a JS coverage delta report
	covjs_stop    # Stop JS coverage collection
	covjs_disable # Alias for Profiler.disable {}

	# CSS Coverage Aliases (CSS Domain)
	covcss_enable  # Alias for CSS.enable {}
	covcss_start   # Start CSS rule usage tracking
	covcss_take    # Take a CSS coverage delta report
	covcss_stop    # Stop CSS rule usage tracking (returns final delta)
	covcss_disable # Alias for CSS.disable {}

Debugging Workflow Example:

 1. `cdp> Page.navigate {"url":"your_test_page.html"}`
 2. Set breakpoints using `Debugger.setBreakpoint` or `Debugger.setBreakpointByUrl`.
 3. Interact with the page (e.g., `Runtime.evaluate {"expression":"document.querySelector('button').click()"}`).
 4. When paused (indicated by `<-- Paused: Debugger.paused` event), inspect state:
    `cdp> Runtime.getProperties {"objectId": "..."}` (get objectId from paused event details)
 5. Use stepping aliases: `step`, `next`, `out`.
 6. Resume execution: `cont`.

Coverage Workflow Example (JS):

1.  `cdp> Page.navigate {"url":"your_test_page.html"}`
2.  `cdp> covjs_start`
3.  Interact with the page/features you want to measure.
4.  `cdp> covjs_take` (View coverage collected so far)
5.  Continue interaction...
6.  `cdp> covjs_stop` (View final coverage report)
7.  `cdp> covjs_disable`

Output:

  - Successful command results are printed as pretty-formatted JSON.
  - Errors from Chrome are printed with error codes and messages.
  - Asynchronous events received from Chrome (e.g., Network.requestWillBeSent)
    are printed prefixed with "<-- Event:".
  - Debugger paused/resumed events are highlighted with "<-- Paused:" / "<-- Resumed".

Exiting:

Type 'exit', 'quit', or press Ctrl+D (EOF) to close the connection and exit cdp.
Press Ctrl+C to send an interrupt signal, which also triggers shutdown.
*/
package main
