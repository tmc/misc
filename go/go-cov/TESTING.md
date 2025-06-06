# Local Testing Guide

## Quick Start

### 1. Start the server
```bash
make server
```

This starts the go-cov service on `http://localhost:8080`

### 2. Test the endpoints

```bash
# Test module discovery  
curl 'http://localhost:8080/go-cov1.24.3?go-get=1'

# Test version info
curl 'http://localhost:8080/go-cov1.24.3/@v/latest.info'

# Test module file
curl 'http://localhost:8080/go-cov1.24.3/@v/latest.mod'

# Download generated installer
curl 'http://localhost:8080/go-cov1.24.3/@v/latest.zip' -o installer.zip
```

### 3. Test the generated installer

```bash
# Extract the installer
unzip installer.zip

# Check the generated code
cat main.go  # Should show version = "go1.24.3"

# Compile the installer 
go build main.go

# Run the installer (will fail without network - expected)
./main
```

## Testing Options

### Automated Testing
```bash
# Run scripttest suite
make test

# Run local integration test
make test-local
```

### Manual Testing
```bash
# Show manual testing commands
make test-manual

# Test with restricted PATH (like scripttest)
make test-restricted
```

### Environment Variables

Set these for testing:

```bash
# Change server port
export PORT=9090

# Test with restricted PATH
export PATH="$(go env GOROOT)/bin"

# Test different Go versions
curl 'http://localhost:8080/go-cov1.22.5/@v/latest.zip' -o installer-go1.22.5.zip
```

## Testing Different Versions

The service dynamically generates installers for any Go version:

```bash
# Test various versions
curl 'http://localhost:8080/go-cov1.21.0/@v/latest.zip' -o go1.21.0.zip
curl 'http://localhost:8080/go-cov1.22.5/@v/latest.zip' -o go1.22.5.zip  
curl 'http://localhost:8080/go-cov1.23.4/@v/latest.zip' -o go1.23.4.zip
curl 'http://localhost:8080/go-cov1.24.3/@v/latest.zip' -o go1.24.3.zip

# Each should contain the correct version in main.go
unzip -q go1.21.0.zip && grep 'version.*=' main.go
```

## Simulating Real Usage

To test the full `go run` workflow locally, you'd need to:

1. Set up a local module proxy or use replace directives
2. Point `GOPROXY` to your local server
3. Use `go run go.tmc.dev/go-cov1.24.3@latest`

For now, the manual testing approach above is simpler and tests the same functionality.

## Expected Behavior

✅ **Server starts** on port 8080  
✅ **Module discovery** returns proper meta tags  
✅ **Version info** returns JSON with version and time  
✅ **Module file** returns valid go.mod with correct module name  
✅ **Zip generation** creates installer with embedded version  
✅ **Installer compiles** even with restricted PATH  
✅ **Installer fails gracefully** without network access  

## Troubleshooting

### Server won't start
- Check if port 8080 is in use: `lsof -i :8080`
- Try different port: `PORT=9090 make server`

### Installer won't compile  
- Check PATH includes Go: `which go`
- Verify Go installation: `go version`

### Network errors in installer
- Expected behavior - installer tries to download real Go releases
- For testing, we just verify it fails gracefully with proper error message