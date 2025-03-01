package validate

import (
	"fmt"
	"regexp"
)

var versionRegex = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[\w\.]+)?$`)

// VersionFormat validates that a version string follows the required format:
// v1.2.3 or v1.2.3-beta.1
func VersionFormat(version string) error {
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s. Must be like v1.2.3 or v1.2.3-beta.1", version)
	}
	return nil
}