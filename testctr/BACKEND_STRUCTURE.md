# Backend Structure Proposal

## Current Structure
```
testctr/
├── testctr-dockerclient/     # Separate module
└── testctr-testcontainers/   # Separate module
```

## Proposed Options

### Option 1: Backends as Subpackages (Recommended)
```
testctr/
├── backend/                   # Interface definition
├── backends/                  # All backend implementations
│   ├── cli/                  # Default CLI backend (extracted from main)
│   │   ├── cli.go
│   │   └── cli_test.go
│   ├── dockerclient/         # Docker SDK backend
│   │   ├── dockerclient.go
│   │   ├── dockerclient_test.go
│   │   └── doc.go
│   └── testcontainers/       # Testcontainers backend
│       ├── testcontainers.go
│       ├── testcontainers_test.go
│       └── doc.go
└── testctrtest/              # Backend testing framework
```

**Pros:**
- Single module, easier versioning
- Clear organization
- Users can import only what they need: `import _ "github.com/tmc/misc/testctr/backends/dockerclient"`
- No separate go.mod files

**Cons:**
- Main module includes all backend dependencies (but with Go modules, unused deps aren't downloaded)

### Option 2: Contrib Directory
```
testctr/
├── backend/                   # Interface
├── internal/cli/             # Built-in CLI backend
└── contrib/                  # External backends
    ├── dockerclient/         # Separate go.mod
    └── testcontainers/       # Separate go.mod
```

**Pros:**
- Core stays minimal
- Clear separation of built-in vs external

**Cons:**
- Multiple modules to maintain
- Version synchronization challenges

### Option 3: Separate Repository
```
github.com/tmc/testctr              # Core library
github.com/tmc/testctr-backends     # All backends
├── dockerclient/
├── testcontainers/
└── ...future backends
```

**Pros:**
- Complete separation
- Independent release cycles

**Cons:**
- More complex for users
- Harder to keep in sync

## Migration Plan for Option 1 (Recommended)

1. Create `backends/` directory structure
2. Extract CLI backend from main package to `backends/cli`
3. Move dockerclient to `backends/dockerclient`
4. Move testcontainers to `backends/testcontainers`
5. Update imports in tests
6. Remove old directories

## Usage After Migration

```go
import (
    "github.com/tmc/misc/testctr"
    _ "github.com/tmc/misc/testctr/backends/dockerclient"
)

func TestWithDockerClient(t *testing.T) {
    c := testctr.New(t, "redis:7",
        testctr.WithBackend("dockerclient"),
    )
}
```