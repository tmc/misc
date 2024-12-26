package chromeprofiles

// ProfileManager handles Chrome profile operations
type ProfileManager interface {
	ListProfiles() ([]string, error)
	SetupWorkdir() error
	Cleanup() error
	CopyProfile(name string, cookieDomains []string) error
	WorkDir() string
}
