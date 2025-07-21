//go:build darwin
// +build darwin

package limits

// setProcLimit is a no-op on macOS
func setProcLimit(maxProcesses uint64) error {
	// Process limits are not supported on macOS in the same way as Linux
	return nil
}