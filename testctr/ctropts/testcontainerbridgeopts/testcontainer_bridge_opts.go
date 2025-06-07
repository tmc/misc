package testcontainerbridgeopts

import (
	"github.com/tmc/misc/testctr"
)

// TestcontainersCustomizer is a function that can customize testcontainers-specific settings
// This is used when the testcontainers backend is selected
type TestcontainersCustomizer func(interface{})

// WithTestcontainersCustomizer allows users to apply testcontainers-specific customizations
// when using the testcontainers backend. The customizer function receives the backend-specific
// configuration object.
//
// Example:
//
//	container := testctr.New(t, "redis:7",
//	    ctropts.WithBackend("testcontainers"),
//	    ctropts.WithTestcontainersCustomizer(func(cfg interface{}) {
//	        // Apply testcontainers-specific customizations
//	        // cfg will be the testcontainers.GenericContainerRequest
//	    }),
//	)
func WithTestcontainersCustomizer(customizer TestcontainersCustomizer) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type testcontainersCustomizerSetter interface {
			AddTestcontainersCustomizer(interface{})
		}
		if tc, ok := cfg.(testcontainersCustomizerSetter); ok {
			tc.AddTestcontainersCustomizer(customizer)
		}
	})
}

// Common testcontainers customizations as convenience functions

// WithTestcontainersPrivileged sets the container to run in privileged mode
// Only works with testcontainers backend
func WithTestcontainersPrivileged() testctr.Option {
	return WithTestcontainersCustomizer(func(cfg interface{}) {
		type privilegedSetter interface {
			SetTestcontainersPrivileged(bool)
		}
		if ps, ok := cfg.(privilegedSetter); ok {
			ps.SetTestcontainersPrivileged(true)
		}
	})
}

// WithTestcontainersAutoRemove sets the container to be automatically removed
// Only works with testcontainers backend
func WithTestcontainersAutoRemove(autoRemove bool) testctr.Option {
	return WithTestcontainersCustomizer(func(cfg interface{}) {
		type autoRemoveSetter interface {
			SetAutoRemove(bool)
		}
		if ar, ok := cfg.(autoRemoveSetter); ok {
			ar.SetAutoRemove(autoRemove)
		}
	})
}

// WithTestcontainersWaitStrategy allows setting a custom wait strategy
// Only works with testcontainers backend
func WithTestcontainersWaitStrategy(strategy interface{}) testctr.Option {
	return WithTestcontainersCustomizer(func(cfg interface{}) {
		type waitStrategySetter interface {
			SetWaitStrategy(interface{})
		}
		if ws, ok := cfg.(waitStrategySetter); ok {
			ws.SetWaitStrategy(strategy)
		}
	})
}

// WithTestcontainersHostConfigModifier allows modifying the Docker host config
// Only works with testcontainers backend
func WithTestcontainersHostConfigModifier(modifier func(interface{})) testctr.Option {
	return WithTestcontainersCustomizer(func(cfg interface{}) {
		type hostConfigModifierSetter interface {
			SetHostConfigModifier(func(interface{}))
		}
		if hm, ok := cfg.(hostConfigModifierSetter); ok {
			hm.SetHostConfigModifier(modifier)
		}
	})
}

// WithTestcontainersReaper allows configuring the Ryuk reaper settings
// Only works with testcontainers backend
func WithTestcontainersReaper(skipReaper bool) testctr.Option {
	return WithTestcontainersCustomizer(func(cfg interface{}) {
		type reaperSetter interface {
			SetSkipReaper(bool)
		}
		if rs, ok := cfg.(reaperSetter); ok {
			rs.SetSkipReaper(skipReaper)
		}
	})
}

// WithTestcontainersReuse configures Testcontainers-Go to reuse a container if one
// with the same configuration and reuseKey is already running. If set to true,
// it also implies skipping the Ryuk reaper for this container.
// This option is a no-op for other backends.
func WithTestcontainersReuse(reuse bool) testctr.Option {
	return WithTestcontainersCustomizer(func(reqRaw interface{}) {
		type reuseSetter interface {
			SetReuse(reuse bool)
		}
		if req, ok := reqRaw.(reuseSetter); ok {
			req.SetReuse(reuse)
		}
	})
}
