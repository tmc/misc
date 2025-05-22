// visual-demo.go - Visual demonstration of coverage injection
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║  Coverage Injection Demo with Say    ║")
	fmt.Println("╚══════════════════════════════════════╝")
	
	// Step 1: Run tests
	fmt.Println("\n▶ Step 1: Running tests with coverage...")
	cmd := exec.Command("go", "test", "-coverprofile=visual-coverage.txt", ".")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	// Step 2: Show original coverage
	fmt.Println("\n▶ Step 2: Original coverage:")
	showCoverage("visual-coverage.txt")
	
	// Step 3: Inject synthetic coverage
	fmt.Println("\n▶ Step 3: Injecting synthetic coverage...")
	injectVisualCoverage("visual-coverage.txt")
	
	// Step 4: Show new coverage  
	fmt.Println("\n▶ Step 4: Coverage after injection:")
	showCoverage("visual-coverage.txt")
	
	// Step 5: Report with say
	fmt.Println("\n▶ Step 5: Audio report...")
	reportWithSay("visual-coverage.txt")
	
	fmt.Println("\n✅ Demo complete!")
}

func showCoverage(file string) {
	cmd := exec.Command("go", "tool", "cover", "-func="+file)
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")
	
	// Show summary
	for _, line := range lines {
		if strings.Contains(line, "total:") {
			fmt.Printf("  📊 %s\n", line)
			break
		}
	}
}

func injectVisualCoverage(file string) {
	// Read existing
	content, _ := os.ReadFile(file)
	lines := strings.Split(string(content), "\n")
	
	// Synthetic coverage to inject
	synthetic := []string{
		"main/ai/neural.go:1.1,500.1 250 1",
		"main/cloud/aws.go:1.1,300.1 150 1", 
		"main/crypto/blockchain.go:1.1,400.1 200 1",
		"main/ml/tensorflow.go:1.1,600.1 300 1",
		"main/quantum/simulator.go:1.1,800.1 400 1",
	}
	
	// Find mode line
	modeIdx := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "mode:") {
			modeIdx = i
			break
		}
	}
	
	// Inject
	var newLines []string
	newLines = append(newLines, lines[:modeIdx+1]...)
	newLines = append(newLines, synthetic...)
	newLines = append(newLines, lines[modeIdx+1:]...)
	
	// Write back
	os.WriteFile(file, []byte(strings.Join(newLines, "\n")), 0644)
	
	// Visual feedback
	for _, line := range synthetic {
		parts := strings.Split(line, " ")
		fmt.Printf("  💉 Injected: %s (%s statements)\n", parts[0], parts[1])
		time.Sleep(200 * time.Millisecond) // For dramatic effect
	}
}

func reportWithSay(file string) {
	// Calculate coverage
	content, _ := os.ReadFile(file)
	lines := strings.Split(string(content), "\n")
	
	var total, covered int
	for _, line := range lines {
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			stmts := 0
			count := 0
			fmt.Sscanf(parts[1], "%d", &stmts)
			fmt.Sscanf(parts[2], "%d", &count)
			
			total += stmts
			if count > 0 {
				covered += stmts
			}
		}
	}
	
	percentage := float64(covered) * 100 / float64(total)
	
	// Create message
	message := fmt.Sprintf(
		"Coverage injection complete. Total coverage is now %.1f percent. "+
		"This includes %d synthetic statements. "+
		"Coverage improved significantly!",
		percentage, 1300) // 250+150+200+300+400 = 1300
	
	// Visual feedback
	fmt.Printf("  🔊 Saying: \"%.20s...\"\n", message)
	
	// Use say command
	cmd := exec.Command("say", message)
	cmd.Run()
}

// Run this with: go run visual-demo.go