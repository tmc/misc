package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/tmc/misc/macgo"
)

func init() {
	macgo.RequestEntitlements(
		macgo.EntUserSelectedReadWrite,
		macgo.EntAppSandbox,
	)
	macgo.Start()
}

func main() {
	h, _ := os.UserHomeDir()
	tccDbPath := h + "/Library/Application Support/com.apple.TCC/TCC.db"
	tccDbPath = "/Library/Application Support/com.apple.TCC/TCC.db"
	fmt.Println(tccDbPath)

	if _, err := os.Stat(tccDbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "TCC.db not found at %s\n", tccDbPath)
		os.Exit(1)
	}
	// read it and cat it into sqlite3:

	f, err := os.Open(tccDbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading TCC.db: %v\n", err)
		// TODO: cehck if macos TCC deny:
		os.Exit(1)
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
	// TODO: read schema
	cmd := exec.Command("sqlite3", tf.Name(), "-json", "select * from access;")
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

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	tw.Write([]byte("Service\tClient\tClientType\tFlags\tAuthReason\tAuthValue\tLastModified\n"))
	for _, tcc := range tccAccess {
		//fmt.Printf("%+v\n", tcc)
		fmt.Fprintf(tw, "%s\t%s\t%f\t%f\t%f\t%f\t%f\n", tcc.Service, tcc.Client, tcc.ClientType, tcc.Flags, tcc.AuthReason, tcc.AuthValue, tcc.LastModified)
	}
	tw.Flush()
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
