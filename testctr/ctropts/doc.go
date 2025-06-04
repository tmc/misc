// Package ctropts provides advanced configuration options for testctr containers.
//
// This package contains all non-essential [github.com/tmc/misc/testctr.Option] functions,
// keeping the core testctr API minimal while providing powerful customization capabilities.
//
// # Container Runtime Options
//
// Control container runtime behavior:
//
//	container := testctr.New(t, "nginx",
//	    ctropts.WithRuntime("podman"),           // Use specific runtime
//	    ctropts.WithPlatform("linux/arm64"),     // Target platform/architecture
//	    ctropts.WithPrivileged(),                // Run in privileged mode
//	    ctropts.WithUser("1000:1000"),           // Set container user
//	    ctropts.WithWorkingDir("/app"),          // Set working directory
//	)
//
// # Network Configuration
//
// Advanced networking options:
//
//	container := testctr.New(t, "app",
//	    ctropts.WithNetwork("custom-network"),   // Join specific network
//	    ctropts.WithHostname("myapp"),           // Set container hostname
//	    ctropts.WithExposedPorts("8080", "9090"), // Expose additional ports
//	    ctropts.WithPublishAllPorts(),           // Publish all exposed ports
//	)
//
// # Volume and File Management
//
// Mount volumes and manage files:
//
//	container := testctr.New(t, "app",
//	    ctropts.WithVolume("/host/data:/container/data"), // Bind mount
//	    ctropts.WithTmpfs("/tmp"),                        // Tmpfs mount
//	    ctropts.WithVolumesFrom("other-container"),       // Mount from container
//	)
//
// Use the core package's file options for simple file operations:
// [github.com/tmc/misc/testctr.WithFile], [github.com/tmc/misc/testctr.WithFiles]
//
// # Wait Strategies
//
// Control when containers are considered "ready":
//
//	container := testctr.New(t, "postgres:15",
//	    ctropts.WithLogWait("database system is ready", 30*time.Second),
//	    ctropts.WithExecWait([]string{"pg_isready"}, 15*time.Second),
//	    ctropts.WithHTTPWait("http://localhost:8080/health", 60*time.Second),
//	)
//
// # Resource Limits
//
// Set container resource constraints:
//
//	container := testctr.New(t, "cpu-intensive-app",
//	    ctropts.WithMemoryLimit("512m"),         // Memory limit
//	    ctropts.WithCPULimit("0.5"),             // CPU limit (0.5 cores)
//	    ctropts.WithShmSize("128m"),             // Shared memory size
//	)
//
// # Database-Specific Options
//
// Specialized database packages provide [github.com/tmc/misc/testctr.DSNProvider]
// functionality and database-specific configurations:
//
//	import "github.com/tmc/misc/testctr/ctropts/postgres"
//	import "github.com/tmc/misc/testctr/ctropts/mysql" 
//	import "github.com/tmc/misc/testctr/ctropts/redis"
//
//	// PostgreSQL with custom configuration
//	pg := testctr.New(t, "postgres:15",
//	    postgres.Default(),                       // Standard postgres setup
//	    postgres.WithDatabase("myapp"),           // Custom database name
//	    postgres.WithUser("testuser", "testpass"), // Custom credentials
//	    ctropts.WithEnv("POSTGRES_INITDB_ARGS", "--auth-host=trust"),
//	)
//	dsn := pg.DSN(t) // Get connection string
//
// # Backend Integration
//
// Options for specific container backends:
//
//	// Testcontainers-go backend options
//	container := testctr.New(t, "app",
//	    ctropts.WithTestcontainersCustomizer(func(req *testcontainers.GenericContainerRequest) {
//	        req.Privileged = true
//	        req.CapAdd = []string{"SYS_ADMIN"}
//	    }),
//	    ctropts.WithTestcontainersPrivileged(),   // Convenience function
//	    ctropts.WithTestcontainersAutoRemove(),   // Auto-remove container
//	)
//
// # Logging and Debugging
//
// Control container logging behavior:
//
//	container := testctr.New(t, "app",
//	    ctropts.WithLogs(),                       // Stream logs to test output
//	    ctropts.WithLogLevel("debug"),            // Set log level
//	    ctropts.WithLogDriver("json-file"),       // Set log driver
//	)
//
// # Environment and Security
//
// Advanced environment and security options:
//
//	container := testctr.New(t, "app",
//	    ctropts.WithEnvFile(".env"),              // Load environment from file
//	    ctropts.WithSecurityOpt("seccomp=unconfined"), // Security options
//	    ctropts.WithCapAdd("NET_ADMIN"),          // Add capabilities
//	    ctropts.WithCapDrop("ALL"),               // Drop capabilities
//	)
//
// # Platform and Architecture Support
//
// testctr supports multi-platform containers through the [WithPlatform] option:
//
//	// Run ARM64 container on any host architecture
//	arm64Container := testctr.New(t, "nginx",
//	    ctropts.WithPlatform("linux/arm64"))
//
//	// Run AMD64 container explicitly  
//	amd64Container := testctr.New(t, "nginx",
//	    ctropts.WithPlatform("linux/amd64"))
//
// Platform emulation is handled automatically by Docker/Podman when the target
// platform differs from the host architecture.
//
// # Option Composition
//
// All ctropts functions return [github.com/tmc/misc/testctr.Option] and can be
// freely mixed with core package options:
//
//	container := testctr.New(t, "complex-app",
//	    // Core options
//	    testctr.WithPort("8080"),
//	    testctr.WithEnv("ENV", "test"),
//	    testctr.WithCommand("server", "--port=8080"),
//	    
//	    // Advanced options
//	    ctropts.WithPlatform("linux/arm64"),
//	    ctropts.WithMemoryLimit("1g"),
//	    ctropts.WithLogWait("Server started", 30*time.Second),
//	    ctropts.WithVolume("/tmp/data:/app/data"),
//	)
package ctropts