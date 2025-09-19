package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tmc/misc/macgo/v2"
)

// getVersion returns the application version from environment or default
func getVersion() string {
	if v := os.Getenv("TEE_SEE_SEE_VERSION"); v != "" {
		return v
	}
	return "1.0.0"
}

func init() {
	// Check if we should trigger TCC prompt
	if len(os.Args) > 1 && os.Args[1] == "--trigger-tcc" {
		// Just try to access TCC database to trigger prompt
		tccPath := "/Library/Application Support/com.apple.TCC/TCC.db"
		if _, err := os.Open(tccPath); err != nil {
			fmt.Fprintf(os.Stderr, "Full Disk Access required. Please grant access in the dialog that appears.\n")
		}
		os.Exit(0)
	}

	// Check if we're in write mode by looking for write-related flags
	isWriteMode := false
	for _, arg := range os.Args[1:] {
		if arg == "-write" || arg == "--write" || arg == "-grant" || arg == "-revoke" || arg == "-reset" {
			isWriteMode = true
			break
		}
	}

	// Use the binary name as the base app name
	appName := filepath.Base(os.Args[0])

	// For write mode, append -writer suffix if not already present
	if isWriteMode && !strings.HasSuffix(appName, "-writer") {
		appName = appName + "-writer"
	}


	cfg := &macgo.Config{
		AppName:               appName,
		BundleID:              "github.com.tmc.misc.tee-see-see." + appName, // Unique bundle ID per mode
		Version:               getVersion(),                                 // Application version
		CodeSigningIdentifier: "github.com.tmc.misc.tee-see-see." + appName, // Unique signing identifier per mode
		Permissions:           []macgo.Permission{macgo.Files},
		Custom: []string{
			"com.apple.security.app-sandbox",
			"com.apple.security.files.user-selected.read-write",
			"com.apple.security.files.downloads.read-write",
			"com.apple.security.files.bookmarks.app-scope",
			"com.apple.security.system-data",
		},
		ForceLaunchServices: true, // Use 'open' command to properly register with TCC
		Debug:               os.Getenv("MACGO_DEBUG") == "1",
		AdHocSign:           true, // Enable ad-hoc code signing
	}
	macgo.Start(cfg)
}

func main() {
	// Define command-line flags
	var (
		useSystem = flag.Bool("system", false, "Use system TCC database instead of user database")
		useUser   = flag.Bool("user", false, "Use user TCC database (default)")
		dbPath    = flag.String("db", "", "Specify custom TCC database path")
		help      = flag.Bool("help", false, "Show help message")
		verbose   = flag.Bool("v", false, "Verbose output")
		jsonOut   = flag.Bool("json", false, "Output in JSON format")
		service   = flag.String("service", "", "Filter by service name")
		client    = flag.String("client", "", "Filter by client/app name")
		waitFDA   = flag.Bool("wait", true, "Wait for Full Disk Access to be granted")
		timeout   = flag.Duration("timeout", 30*time.Second, "Timeout for waiting (e.g., 30s, 5m)")
		appVersion = flag.String("app-version", "", "Set application version (default: 1.0.0)")

		// Write mode flags (triggers separate app bundle)
		grant    = flag.String("grant", "", "Grant access: -grant 'service:client' (uses tee-see-see-writer.app)")
		revoke   = flag.String("revoke", "", "Revoke access: -revoke 'service:client' (uses tee-see-see-writer.app)")
		reset    = flag.String("reset", "", "Reset client permissions: -reset 'client' (uses tee-see-see-writer.app)")
		writeSQL = flag.String("write", "", "Execute custom SQL: -write 'UPDATE ...' (uses tee-see-see-writer.app)")
	)

	// Parse flags
	flag.Parse()

	// Set version environment variable if flag is provided
	if *appVersion != "" {
		os.Setenv("TEE_SEE_SEE_VERSION", *appVersion)
	}


	// Show help if requested
	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Read and display macOS TCC (Transparency, Consent, and Control) database.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nRead Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s              # Use user database (default)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -system      # Use system database\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -wait        # Wait for FDA to be granted\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -client iterm -json # Filter by client, JSON output\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nWrite Examples (uses separate tee-see-see-writer.app):\n")
		fmt.Fprintf(os.Stderr, "  %s -grant 'kTCCServiceCamera:com.example.app'  # Grant camera access\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -revoke 'kTCCServiceCamera:com.example.app' # Revoke camera access\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -reset 'com.example.app'  # Reset all permissions for app\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -write 'DELETE FROM access WHERE client=\"old.app\"'  # Custom SQL\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nNote: Write operations require separate FDA permission for tee-see-see-writer.app\n")
		os.Exit(0)
	}

	// Determine which database to use
	var tccDbPath string

	if *dbPath != "" {
		// Custom path specified
		tccDbPath = *dbPath
	} else if *useSystem {
		// Explicitly use system database
		tccDbPath = "/Library/Application Support/com.apple.TCC/TCC.db"
	} else {
		// Default to user database
		h, _ := os.UserHomeDir()
		tccDbPath = h + "/Library/Application Support/com.apple.TCC/TCC.db"

		// If user database doesn't exist and system wasn't explicitly requested, try system
		if !*useUser {
			if _, err := os.Stat(tccDbPath); os.IsNotExist(err) {
				tccDbPath = "/Library/Application Support/com.apple.TCC/TCC.db"
			}
		}
	}

	if *verbose {
		fmt.Println(tccDbPath)
	}

	// Check if we're doing write operations
	isWriteOperation := *grant != "" || *revoke != "" || *reset != "" || *writeSQL != ""

	if isWriteOperation {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Write mode: using tee-see-see-writer.app bundle\n")
		}
		performWriteOperation(tccDbPath, *grant, *revoke, *reset, *writeSQL, *waitFDA, *timeout, *verbose)
		return
	}

	if _, err := os.Stat(tccDbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "TCC.db not found at %s\n", tccDbPath)
		os.Exit(1)
	}
	// read it and cat it into sqlite3:

	// Try to open the database
	f, err := os.Open(tccDbPath)
	if err != nil {
		if *waitFDA {
			fmt.Fprintf(os.Stderr, "Waiting for Full Disk Access...\n")
			start := time.Now()
			for {
				time.Sleep(2 * time.Second)
				f, err = os.Open(tccDbPath)
				if err == nil {
					break
				}
				if time.Since(start) > *timeout {
					fmt.Fprintf(os.Stderr, "Timeout\n")
					os.Exit(1)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
	defer f.Close()
	// copy to tmp:
	tf, err := os.CreateTemp("", "tcc.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tf.Name())
	defer tf.Close()
	_, err = io.Copy(tf, f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error copying TCC.db: %v\n", err)
		os.Exit(1)
	}

	// Check if sqlite3 is available
	if _, err := exec.LookPath("sqlite3"); err != nil {
		fmt.Fprintf(os.Stderr, "sqlite3 command not found. Please install SQLite.\n")
		os.Exit(1)
	}

	// Build SQL query with filters
	query := "select * from access"
	whereClauses := []string{}

	if *service != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("service LIKE '%%%s%%'", *service))
	}
	if *client != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("client LIKE '%%%s%%'", *client))
	}

	if len(whereClauses) > 0 {
		query += " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				query += " AND "
			}
			query += clause
		}
	}
	query += ";"

	cmd := exec.Command("sqlite3", tf.Name(), "-json", query)
	cmd.Stderr = os.Stderr

	so, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		os.Exit(1)
	}
	err = cmd.Start()
	// read json:
	tccAccess := []TCCAccess{}
	err = json.NewDecoder(so).Decode(&tccAccess)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding JSON: %v\n", err)
		os.Exit(1)
	}

	// Output results
	if *jsonOut {
		// JSON output
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(tccAccess); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON output: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Table output
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		tw.Write([]byte("Service\tClient\tClientType\tFlags\tAuthReason\tAuthValue\tLastModified\n"))
		for _, tcc := range tccAccess {
			fmt.Fprintf(tw, "%s\t%s\t%f\t%f\t%f\t%f\t%f\n", tcc.Service, tcc.Client, tcc.ClientType, tcc.Flags, tcc.AuthReason, tcc.AuthValue, tcc.LastModified)
		}
		tw.Flush()
	}

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing SQLite command: %v\n", err)
		os.Exit(1)
	}

}

type TCCAccess struct {
	AuthReason                   float64     `json:"auth_reason,omitempty"`
	AuthValue                    float64     `json:"auth_value,omitempty"`
	AuthVersion                  float64     `json:"auth_version,omitempty"`
	BootUuid                     string      `json:"boot_uuid,omitempty"`
	Client                       string      `json:"client,omitempty"`
	ClientType                   float64     `json:"client_type,omitempty"`
	Csreq                        *string     `json:"csreq,omitempty"`
	Flags                        float64     `json:"flags,omitempty"`
	IndirectObjectCodeIdentity   interface{} `json:"indirect_object_code_identity,omitempty"`
	IndirectObjectIdentifier     string      `json:"indirect_object_identifier,omitempty"`
	IndirectObjectIdentifierType *float64    `json:"indirect_object_identifier_type,omitempty"`
	LastModified                 float64     `json:"last_modified,omitempty"`
	LastReminded                 float64     `json:"last_reminded,omitempty"`
	Pid                          interface{} `json:"pid,omitempty"`
	PidVersion                   interface{} `json:"pid_version,omitempty"`
	PolicyID                     interface{} `json:"policy_id,omitempty"`
	Service                      string      `json:"service,omitempty"`
}

// performWriteOperation handles TCC database write operations
func performWriteOperation(tccDbPath, grant, revoke, reset, writeSQL string, waitFDA bool, timeout time.Duration, verbose bool) {
	// Check if sqlite3 is available
	if _, err := exec.LookPath("sqlite3"); err != nil {
		fmt.Fprintf(os.Stderr, "sqlite3 command not found. Please install SQLite.\n")
		os.Exit(1)
	}

	// Test database access
	testCmd := exec.Command("sqlite3", tccDbPath, "SELECT COUNT(*) FROM access LIMIT 1;")
	testCmd.Stderr = nil
	testCmd.Stdout = nil

	if err := testCmd.Run(); err != nil {
		if waitFDA {
			fmt.Fprintf(os.Stderr, "Waiting for writer access...\n")
			start := time.Now()
			for {
				time.Sleep(2 * time.Second)
				if testCmd.Run() == nil {
					break
				}
				if time.Since(start) > timeout {
					fmt.Fprintf(os.Stderr, "Timeout\n")
					os.Exit(1)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Build SQL command
	var sqlCmd string

	if grant != "" {
		parts := strings.SplitN(grant, ":", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: -grant format should be 'service:client'\n")
			os.Exit(1)
		}
		service, client := parts[0], parts[1]
		sqlCmd = fmt.Sprintf("INSERT OR REPLACE INTO access (service, client, client_type, auth_value, auth_reason, auth_version, flags, last_modified) VALUES ('%s', '%s', 0, 2, 2, 1, 0, %d);",
			service, client, time.Now().Unix())
	} else if revoke != "" {
		parts := strings.SplitN(revoke, ":", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: -revoke format should be 'service:client'\n")
			os.Exit(1)
		}
		service, client := parts[0], parts[1]
		sqlCmd = fmt.Sprintf("UPDATE access SET auth_value = 0, last_modified = %d WHERE service = '%s' AND client = '%s';",
			time.Now().Unix(), service, client)
	} else if reset != "" {
		sqlCmd = fmt.Sprintf("DELETE FROM access WHERE client = '%s';", reset)
	} else if writeSQL != "" {
		sqlCmd = writeSQL
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Executing SQL: %s\n", sqlCmd)
	}

	// Execute the SQL command
	cmd := exec.Command("sqlite3", tccDbPath, sqlCmd)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing SQL: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "✅ TCC database updated successfully\n")
}
