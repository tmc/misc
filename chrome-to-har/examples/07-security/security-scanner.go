// Security testing example
// This example shows how to perform basic security testing using chrome-to-har
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type SecurityTestResult struct {
	URL           string                 `json:"url"`
	Timestamp     time.Time              `json:"timestamp"`
	Success       bool                   `json:"success"`
	SecurityScore int                    `json:"security_score"`
	Findings      []SecurityFinding      `json:"findings"`
	HTTPSCheck    HTTPSResult            `json:"https_check"`
	HeaderCheck   HeaderSecurity         `json:"header_security"`
	ContentCheck  ContentSecurity        `json:"content_security"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type SecurityFinding struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}

type HTTPSResult struct {
	IsHTTPS    bool   `json:"is_https"`
	HasMixedContent bool `json:"has_mixed_content"`
	CertificateValid bool `json:"certificate_valid"`
}

type HeaderSecurity struct {
	HasCSP             bool `json:"has_csp"`
	HasXFrameOptions   bool `json:"has_x_frame_options"`
	HasXSSProtection   bool `json:"has_xss_protection"`
	HasHSTS            bool `json:"has_hsts"`
	HasContentTypeOptions bool `json:"has_content_type_options"`
}

type ContentSecurity struct {
	HasInlineScript    bool `json:"has_inline_script"`
	HasInlineStyle     bool `json:"has_inline_style"`
	HasExternalScript  bool `json:"has_external_script"`
	VulnerablePatterns []string `json:"vulnerable_patterns"`
}

type NetworkStats struct {
	TotalRequests    int `json:"total_requests"`
	HTTPSRequests    int `json:"https_requests"`
	HTTPRequests     int `json:"http_requests"`
	ThirdPartyRequests int `json:"third_party_requests"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run security-scanner.go <url>")
		fmt.Println("Example: go run security-scanner.go https://example.com")
		os.Exit(1)
	}

	url := os.Args[1]
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Chrome browser for security testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; SecurityScanner/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("ignore-certificate-errors", false), // Don't ignore cert errors for security testing
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Run security scan
	result := performSecurityScan(chromeCtx, url)
	
	// Display results
	displaySecurityResults(result)
	
	// Save report
	saveSecurityReport(result)
	
	// Exit with appropriate code
	if result.SecurityScore < 70 {
		os.Exit(1)
	}
}

func performSecurityScan(chromeCtx context.Context, url string) SecurityTestResult {
	startTime := time.Now()
	
	result := SecurityTestResult{
		URL:       url,
		Timestamp: startTime,
		Success:   false,
		Findings:  []SecurityFinding{},
	}

	// Create recorder for network analysis
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 60*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content
		
		// Check HTTPS
		chromedp.Evaluate(`
			(function() {
				return {
					isHTTPS: window.location.protocol === 'https:',
					hasMixedContent: document.querySelectorAll('img[src^="http:"], script[src^="http:"], link[href^="http:"]').length > 0,
					certificateValid: true // This would need deeper inspection
				};
			})()
		`, &result.HTTPSCheck),
		
		// Check security headers
		chromedp.Evaluate(`
			(function() {
				const headers = {};
				
				// Check for CSP
				const cspMeta = document.querySelector('meta[http-equiv="Content-Security-Policy"]');
				headers.hasCSP = !!cspMeta;
				
				// Check for X-Frame-Options (this would be in response headers, not accessible via JS)
				// In a real implementation, you'd analyze the HAR file for these headers
				
				return headers;
			})()
		`, &result.HeaderCheck),
		
		// Check content security
		chromedp.Evaluate(`
			(function() {
				const content = {
					hasInlineScript: document.querySelectorAll('script:not([src])').length > 0,
					hasInlineStyle: document.querySelectorAll('style').length > 0,
					hasExternalScript: document.querySelectorAll('script[src]').length > 0,
					vulnerablePatterns: []
				};
				
				// Check for potential XSS vulnerabilities
				const scripts = document.querySelectorAll('script');
				scripts.forEach(script => {
					const scriptContent = script.textContent || script.innerText;
					if (scriptContent.includes('eval(') || scriptContent.includes('innerHTML')) {
						content.vulnerablePatterns.push('Potentially unsafe JavaScript patterns detected');
					}
				});
				
				// Check for forms without CSRF protection
				const forms = document.querySelectorAll('form');
				forms.forEach(form => {
					const hasCSRFToken = form.querySelector('input[name*="csrf"], input[name*="token"]');
					if (!hasCSRFToken && form.method.toLowerCase() === 'post') {
						content.vulnerablePatterns.push('Form without CSRF protection detected');
					}
				});
				
				return content;
			})()
		`, &result.ContentCheck),
		
		rec.Stop(),
	)

	if err != nil {
		result.Findings = append(result.Findings, SecurityFinding{
			Type:        "Error",
			Severity:    "High",
			Description: fmt.Sprintf("Failed to load page: %v", err),
			Remediation: "Ensure the URL is accessible and valid",
		})
		return result
	}

	// Extract network statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.NetworkStats = extractSecurityNetworkStats(harData, url)
	}

	// Analyze security findings
	result.Findings = analyzeSecurityFindings(result)
	
	// Calculate security score
	result.SecurityScore = calculateSecurityScore(result)
	
	result.Success = true
	return result
}

func analyzeSecurityFindings(result SecurityTestResult) []SecurityFinding {
	var findings []SecurityFinding
	
	// HTTPS checks
	if !result.HTTPSCheck.IsHTTPS {
		findings = append(findings, SecurityFinding{
			Type:        "HTTPS",
			Severity:    "High",
			Description: "Site is not using HTTPS",
			Remediation: "Implement HTTPS with a valid SSL certificate",
		})
	}
	
	if result.HTTPSCheck.HasMixedContent {
		findings = append(findings, SecurityFinding{
			Type:        "Mixed Content",
			Severity:    "Medium",
			Description: "Site has mixed HTTP/HTTPS content",
			Remediation: "Ensure all resources are loaded over HTTPS",
		})
	}
	
	// Content Security Policy
	if !result.HeaderCheck.HasCSP {
		findings = append(findings, SecurityFinding{
			Type:        "CSP",
			Severity:    "Medium",
			Description: "Content Security Policy not implemented",
			Remediation: "Implement a strict Content Security Policy",
		})
	}
	
	// Inline scripts
	if result.ContentCheck.HasInlineScript {
		findings = append(findings, SecurityFinding{
			Type:        "Inline Script",
			Severity:    "Low",
			Description: "Inline JavaScript detected",
			Remediation: "Move inline scripts to external files and use CSP",
		})
	}
	
	// Vulnerable patterns
	for _, pattern := range result.ContentCheck.VulnerablePatterns {
		findings = append(findings, SecurityFinding{
			Type:        "Vulnerability",
			Severity:    "Medium",
			Description: pattern,
			Remediation: "Review and fix identified security issues",
		})
	}
	
	return findings
}

func extractSecurityNetworkStats(harData, baseURL string) NetworkStats {
	// Simple HAR parsing for security-related network stats
	stats := NetworkStats{}
	
	// Count total requests
	stats.TotalRequests = strings.Count(harData, `"request":`)
	
	// Count HTTPS vs HTTP requests
	stats.HTTPSRequests = strings.Count(harData, `"https://`)
	stats.HTTPRequests = strings.Count(harData, `"http://`)
	
	// Count third-party requests (simplified)
	// In a real implementation, you'd parse the HAR JSON and check domains
	stats.ThirdPartyRequests = stats.TotalRequests / 4 // Rough estimate
	
	return stats
}

func calculateSecurityScore(result SecurityTestResult) int {
	score := 100
	
	// Deduct points for findings
	for _, finding := range result.Findings {
		switch finding.Severity {
		case "High":
			score -= 25
		case "Medium":
			score -= 15
		case "Low":
			score -= 5
		}
	}
	
	// Bonus points for good practices
	if result.HTTPSCheck.IsHTTPS {
		score += 10
	}
	
	if result.HeaderCheck.HasCSP {
		score += 10
	}
	
	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	
	return score
}

func displaySecurityResults(result SecurityTestResult) {
	fmt.Printf("Security Scan Results for %s\n", result.URL)
	fmt.Printf("Security Score: %d/100\n", result.SecurityScore)
	
	// Display grade
	grade := "F"
	if result.SecurityScore >= 90 {
		grade = "A"
	} else if result.SecurityScore >= 80 {
		grade = "B"
	} else if result.SecurityScore >= 70 {
		grade = "C"
	} else if result.SecurityScore >= 60 {
		grade = "D"
	}
	fmt.Printf("Grade: %s\n", grade)
	
	// Display HTTPS status
	fmt.Printf("\nHTTPS Status:\n")
	fmt.Printf("  HTTPS: %s\n", boolToStatus(result.HTTPSCheck.IsHTTPS))
	fmt.Printf("  Mixed Content: %s\n", boolToStatus(!result.HTTPSCheck.HasMixedContent))
	
	// Display security headers
	fmt.Printf("\nSecurity Headers:\n")
	fmt.Printf("  CSP: %s\n", boolToStatus(result.HeaderCheck.HasCSP))
	fmt.Printf("  X-Frame-Options: %s\n", boolToStatus(result.HeaderCheck.HasXFrameOptions))
	fmt.Printf("  HSTS: %s\n", boolToStatus(result.HeaderCheck.HasHSTS))
	
	// Display findings
	if len(result.Findings) > 0 {
		fmt.Printf("\nSecurity Findings:\n")
		for i, finding := range result.Findings {
			fmt.Printf("  %d. [%s] %s\n", i+1, finding.Severity, finding.Description)
			fmt.Printf("     Remediation: %s\n", finding.Remediation)
		}
	}
	
	// Display network stats
	fmt.Printf("\nNetwork Statistics:\n")
	fmt.Printf("  Total Requests: %d\n", result.NetworkStats.TotalRequests)
	fmt.Printf("  HTTPS Requests: %d\n", result.NetworkStats.HTTPSRequests)
	fmt.Printf("  HTTP Requests: %d\n", result.NetworkStats.HTTPRequests)
	fmt.Printf("  Third-Party Requests: %d\n", result.NetworkStats.ThirdPartyRequests)
}

func boolToStatus(value bool) string {
	if value {
		return "✓ Pass"
	}
	return "✗ Fail"
}

func saveSecurityReport(result SecurityTestResult) {
	// Save as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	filename := fmt.Sprintf("security-report-%d.json", time.Now().Unix())
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed security report saved to %s\n", filename)
}