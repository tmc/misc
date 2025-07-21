package validation

import (
	"fmt"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid profile name", "Default", false},
		{"Valid profile with spaces", "Profile 1", false},
		{"Valid profile with underscores", "test_profile", false},
		{"Valid profile with hyphens", "test-profile", false},
		{"Empty name", "", true},
		{"Too long name", string(make([]rune, 300)), true},
		{"Directory traversal", "../test", true},
		{"Path separator", "test/bad", true},
		{"Windows path separator", "test\\bad", true},
		{"Reserved name", "CON", true},
		{"Control characters", "test\x00name", true},
		{"Invalid characters", "test<>name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name              string
		url               string
		allowedProtocols  []string
		wantErr          bool
	}{
		{"Valid HTTP URL", "http://example.com", []string{"http", "https"}, false},
		{"Valid HTTPS URL", "https://example.com", []string{"http", "https"}, false},
		{"JavaScript URL", "javascript:alert(1)", []string{"http", "https"}, true},
		{"Data URL", "data:text/html,<script>alert(1)</script>", []string{"http", "https"}, true},
		{"VBScript URL", "vbscript:msgbox(1)", []string{"http", "https"}, true},
		{"Invalid protocol", "ftp://example.com", []string{"http", "https"}, true},
		{"Empty URL", "", []string{"http", "https"}, true},
		{"Control characters", "http://example.com\x00", []string{"http", "https"}, true},
		{"Valid file URL", "file:///etc/passwd", []string{"file"}, false},
		{"Malformed file URL", "file:passwd", []string{"file"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url, tt.allowedProtocols)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{"Valid hostname", "example.com", false},
		{"Valid subdomain", "sub.example.com", false},
		{"Valid IP address", "192.168.1.1", false},
		{"Valid IPv6", "2001:db8::1", false},
		{"Empty hostname", "", true},
		{"Too long hostname", string(make([]rune, 300)), true},
		{"Invalid characters", "exam<ple.com", true},
		{"Starting with hyphen", "-example.com", true},
		{"Ending with hyphen", "example.com-", true},
		{"Consecutive dots", "example..com", true},
		{"Control characters", "example\x00.com", true},
		{"Long label", "a" + string(make([]rune, 70)) + ".com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"Valid port", 8080, false},
		{"High port", 65535, false},
		{"Zero port", 0, true},
		{"Negative port", -1, true},
		{"Port too high", 65536, true},
		{"Privileged port", 80, true},
		{"Low privileged port", 1, true},
		{"High privileged port", 1023, true},
		{"First non-privileged port", 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJavaScript(t *testing.T) {
	tests := []struct {
		name           string
		script         string
		allowDangerous bool
		wantErr        bool
	}{
		{"Valid script", "console.log('hello');", false, false},
		{"Dangerous eval", "eval('alert(1)');", false, true},
		{"Dangerous eval allowed", "eval('alert(1)');", true, false},
		{"Dangerous Function", "Function('alert(1)');", false, true},
		{"Dangerous innerHTML", "element.innerHTML = '<script>alert(1)</script>';", false, true},
		{"Dangerous setTimeout", "setTimeout('alert(1)', 1000);", false, true},
		{"Empty script", "", false, true},
		{"Very long script", string(make([]rune, 2000000)), false, true},
		{"Control characters", "console.log('\x00');", false, true},
		{"Unbalanced braces", "if (true) { console.log('test');", false, true},
		{"Unbalanced parentheses", "console.log('test'", false, true},
		{"Unbalanced brackets", "array[0", false, true},
		{"Complex valid script", "function test() { return 'hello'; }", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJavaScript(tt.script, tt.allowDangerous)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJavaScript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		wantErr bool
	}{
		{"Valid timeout", 30, false},
		{"Maximum timeout", 3600, false},
		{"Zero timeout", 0, true},
		{"Negative timeout", -1, true},
		{"Too long timeout", 3601, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout(tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantErr bool
	}{
		{"Valid headers", map[string]string{"Content-Type": "application/json"}, false},
		{"Empty headers", map[string]string{}, false},
		{"Too many headers", func() map[string]string {
			headers := make(map[string]string)
			for i := 0; i < 101; i++ {
				headers[fmt.Sprintf("Header%d", i)] = "value"
			}
			return headers
		}(), true},
		{"Empty header name", map[string]string{"": "value"}, true},
		{"Long header name", map[string]string{string(make([]rune, 300)): "value"}, true},
		{"Long header value", map[string]string{"Header": string(make([]rune, 10000))}, true},
		{"Control chars in name", map[string]string{"Header\x00": "value"}, true},
		{"Control chars in value", map[string]string{"Header": "value\x00"}, true},
		{"Dangerous header", map[string]string{"Host": "evil.com"}, true},
		{"Tab in value (allowed)", map[string]string{"Header": "value\tpart"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeaders(tt.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHeaders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"Valid filename", "test.txt", "test.txt"},
		{"Invalid characters", "test<>file.txt", "test__file.txt"},
		{"Path traversal", "../test.txt", "test.txt"},
		{"Empty filename", "", "output"},
		{"Only dots", ".", "output"},
		{"Only double dots", "..", "output"},
		{"Hidden file", ".hidden", "file_.hidden"},
		{"Very long filename", string(make([]rune, 300)), string(make([]rune, 255))},
		{"Unicode characters", "tëst.txt", "t_st.txt"},
		{"Spaces", "test file.txt", "test_file.txt"},
		{"Multiple extensions", "test.tar.gz", "test.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.filename)
			if len(tt.expected) == 255 {
				// For very long filenames, just check the length
				if len(result) != 255 {
					t.Errorf("SanitizeFilename() length = %d, expected 255", len(result))
				}
			} else if result != tt.expected {
				t.Errorf("SanitizeFilename() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestValidateRemoteHosts(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		allowedHosts []string
		wantErr      bool
	}{
		{"Valid host in list", "localhost", []string{"localhost", "127.0.0.1"}, false},
		{"Valid IP in list", "127.0.0.1", []string{"localhost", "127.0.0.1"}, false},
		{"Host not in list", "evil.com", []string{"localhost", "127.0.0.1"}, true},
		{"Empty allowed list", "localhost", []string{}, true},
		{"Invalid hostname", "invalid<>host", []string{"localhost"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemoteHosts(tt.host, tt.allowedHosts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemoteHosts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{"Valid HTTP proxy", "http://proxy.example.com:8080", false},
		{"Valid HTTPS proxy", "https://proxy.example.com:8080", false},
		{"Valid SOCKS5 proxy", "socks5://proxy.example.com:1080", false},
		{"Invalid scheme", "ftp://proxy.example.com:8080", true},
		{"Empty URL", "", true},
		{"Invalid hostname", "http://invalid<>host:8080", true},
		{"Invalid port", "http://proxy.example.com:99999", true},
		{"Missing port", "http://proxy.example.com", false},
		{"Malformed URL", "http://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyURL(tt.proxyURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProxyURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		wantErr     bool
	}{
		{"Valid path", "/tmp/test", []string{"/tmp"}, false},
		{"Empty path", "", []string{"/tmp"}, true},
		{"Directory traversal", "/tmp/../etc/passwd", []string{"/tmp"}, true},
		{"Path outside allowed dirs", "/etc/passwd", []string{"/tmp"}, true},
		{"Null bytes", "/tmp/test\x00", []string{"/tmp"}, true},
		{"No allowed dirs", "/tmp/test", []string{}, false},
		{"Current directory", "./test", []string{"."}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.allowedDirs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserAgent(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		wantErr   bool
	}{
		{"Valid user agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", false},
		{"Empty user agent", "", true},
		{"Very long user agent", string(make([]rune, 2000)), true},
		{"Control characters", "Mozilla/5.0\x00", true},
		{"Tab character (allowed)", "Mozilla/5.0\t", false},
		{"Newline character", "Mozilla/5.0\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAgent(tt.userAgent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUserAgent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmarks
func BenchmarkValidateProfileName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateProfileName("Default")
	}
}

func BenchmarkValidateURL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateURL("https://example.com", []string{"http", "https"})
	}
}

func BenchmarkValidateJavaScript(b *testing.B) {
	script := "console.log('hello world');"
	for i := 0; i < b.N; i++ {
		ValidateJavaScript(script, false)
	}
}

func BenchmarkSanitizeFilename(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SanitizeFilename("test<>file.txt")
	}
}