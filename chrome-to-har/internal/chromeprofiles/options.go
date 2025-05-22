package chromeprofiles

// Option configures a profile manager
type Option func(*profileManager)

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(pm *profileManager) {
		pm.verbose = verbose
	}
}

// WithProfileDir sets the base profile directory
func WithProfileDir(dir string) Option {
	return func(pm *profileManager) {
		pm.baseDir = dir
	}
}
