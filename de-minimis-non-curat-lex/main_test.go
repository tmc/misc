package main

import (
	"flag"
	"os"
	"strings"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var flagQuiet = flag.Bool("quiet", false, "suppress output from script engine")

func TestMain(m *testing.M) {
	// Parse command line flags
	flag.Parse()
	// Run the tests
	exitCode := m.Run()
	// Exit with the appropriate code
	os.Exit(exitCode)
}

func TestAgentCoordination(t *testing.T) {
	// Check that agents are following coordination protocol
	chatContent, err := os.ReadFile("agent-chat.txt")
	if err != nil {
		t.Fatalf("Failed to read agent-chat.txt: %v", err)
	}
	
	chat := string(chatContent)
	
	// Check if there are any unacknowledged requests
	lines := strings.Split(chat, "\n")
	var needsAck bool
	
	for _, line := range lines {
		if strings.Contains(line, "Please acknowledge") {
			needsAck = true
		}
		if strings.Contains(line, "ACKNOWLEDGED") {
			needsAck = false
		}
	}
	
	if needsAck && !strings.Contains(chat, "ACKNOWLEDGED") {
		t.Fatal("COORDINATION FAILURE: Agent must acknowledge before proceeding")
	}
	
	// Ensure there's some form of agent coordination happening
	if !strings.Contains(chat, "Agent") {
		t.Fatal("COORDINATION FAILURE: No agent coordination found in agent-chat.txt")
	}
}

func TestAgentConversation(t *testing.T) {
	// Ensure agents respond to direct questions/requests
	chatContent, err := os.ReadFile("agent-chat.txt")
	if err != nil {
		t.Fatalf("Failed to read agent-chat.txt: %v", err)
	}
	
	chat := string(chatContent)
	lines := strings.Split(chat, "\n")
	
	var questionAsker string
	var questionLine int
	var hasUnansweredQuestion bool
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Check who's speaking and if they ask a question
		if strings.HasPrefix(line, "Agent") && strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				speaker := strings.TrimSpace(parts[0])
				content := strings.TrimSpace(parts[1])
				
				// Check for questions or requests that need responses
				if strings.Contains(content, "?") || 
				   strings.Contains(content, "how are you") ||
				   strings.Contains(content, "what do you think") ||
				   strings.Contains(content, "how do you feel") {
					questionAsker = speaker
					questionLine = i
					hasUnansweredQuestion = true
				}
				
				// If someone else responds after the question, mark as answered
				if hasUnansweredQuestion && speaker != questionAsker && i > questionLine {
					hasUnansweredQuestion = false
				}
			}
		}
	}
	
	if hasUnansweredQuestion {
		t.Fatal("CONVERSATION FAILURE: There are unanswered questions in agent-chat.txt - agents must respond to direct questions")
	}
	
	// Check if scripts have been executed when requested
	if strings.Contains(chat, "Please actually run the script") && !strings.Contains(chat, "cgpt-helper.sh output:") {
		t.Fatal("SCRIPT EXECUTION FAILURE: Agent must run requested scripts and show output")
	}
	
	// Check for urgent requests
	if strings.Contains(chat, "URGENT") && !strings.Contains(chat, "Agent2: I will") {
		t.Fatal("URGENT REQUEST FAILURE: Agent must respond to urgent coordination requests immediately")
	}
}

func TestScript(t *testing.T) {
	// Get the current module's root directory
	engine := script.NewEngine()
	//engine.Quiet = !testing.Verbose()
	engine.Cmds["de-minimis-non-curat-lex"] = mainCmd
	// Add echo command for testing
	engine.Cmds["echo"] = script.Echo()
	engine.Quiet = *flagQuiet

	env := []string{}
	// Run all script tests in testdata/script/
	scripttest.Test(t, t.Context(), engine, env, "testdata/*.txt")
}

var mainCmd = script.Command(
	script.CmdUsage{
		Summary: "de-minimis-non-curat-lex",
		Args:    "[files...]",
	},
	func(state *script.State, args ...string) (script.WaitFunc, error) {
		// The script test runs in a temporary directory
		// Files are created there by the test harness
		if testing.Verbose() {
			state.Logf("Running de-minimis-non-curat-lex with args: %v\n", args)
			state.Logf("Working directory: %s\n", state.Getwd())
			// List files
			entries, _ := os.ReadDir(".")
			var files []string
			for _, e := range entries {
				files = append(files, e.Name())
			}
			state.Logf("Files in directory: %v\n", files)
		}
		// Change to the script's working directory
		oldDir, _ := os.Getwd()
		os.Chdir(state.Getwd())
		defer os.Chdir(oldDir)

		err := run(args)
		return nil, err
	},
)
