# Omni Package Format Specifications

This document details the package formats that Omni generates for different ecosystem distributions.

## Common Elements

All packages share these common characteristics:

1. Each package contains platform-specific binaries for the Go tool
2. A wrapper script or module detects the platform and runs the appropriate binary
3. Package metadata follows the conventions of the target ecosystem
4. Installation places the tool in the user's PATH

## Python Package

### Structure

```
omni-<tool>-<version>/
├── setup.py
├── pyproject.toml
├── <tool>/
│   ├── __init__.py   # Contains main() function for command-line entrypoint
│   └── bin/          # Platform-specific binaries
│       ├── darwin_amd64/<tool>
│       ├── darwin_arm64/<tool>
│       ├── linux_amd64/<tool>
│       ├── linux_arm64/<tool>
│       └── windows_amd64/<tool>.exe
```

### Key Files

#### setup.py

```python
from setuptools import setup, find_packages

setup(
    name="<tool>",
    version="<version>",
    description="<description>",
    author="<author>",
    author_email="<email>",
    url="<repository_url>",
    packages=find_packages(),
    include_package_data=True,
    package_data={
        "<tool>": ["bin/**/*"],
    },
    entry_points={
        "console_scripts": [
            "<tool>=<tool>:main",
        ],
    },
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        # ...
    ],
)
```

#### pyproject.toml

```toml
[build-system]
requires = ["setuptools>=42", "wheel"]
build-backend = "setuptools.build_meta"
```

#### __init__.py

```python
import os
import subprocess
import sys
from pathlib import Path

def main():
    # Find the binary for the current platform
    bin_dir = Path(__file__).parent / "bin"
    
    # Platform detection logic
    import platform
    system = platform.system().lower()
    machine = platform.machine().lower()
    
    if system == "darwin":
        if machine in ["x86_64", "amd64"]:
            binary_path = bin_dir / "darwin_amd64" / "<tool>"
        elif machine in ["arm64", "aarch64"]:
            binary_path = bin_dir / "darwin_arm64" / "<tool>"
    elif system == "linux":
        if machine in ["x86_64", "amd64"]:
            binary_path = bin_dir / "linux_amd64" / "<tool>"
        elif machine in ["arm64", "aarch64"]:
            binary_path = bin_dir / "linux_arm64" / "<tool>"
    elif system == "windows":
        binary_path = bin_dir / "windows_amd64" / "<tool>.exe"
    else:
        sys.stderr.write(f"Unsupported platform: {system}_{machine}\n")
        sys.exit(1)
    
    if not binary_path.exists():
        sys.stderr.write(f"Binary not found: {binary_path}\n")
        sys.exit(1)
    
    # Make sure the binary is executable
    binary_path.chmod(0o755)
    
    # Execute the binary with the same arguments
    result = subprocess.run([str(binary_path)] + sys.argv[1:])
    sys.exit(result.returncode)

if __name__ == "__main__":
    main()
```

## Node.js Package

### Structure

```
<tool>-<version>/
├── package.json
├── index.js    # Contains wrapper code
└── bin/        # Platform-specific binaries
    ├── darwin_amd64/<tool>
    ├── darwin_arm64/<tool>
    ├── linux_amd64/<tool>
    ├── linux_arm64/<tool>
    └── windows_amd64/<tool>.exe
```

### Key Files

#### package.json

```json
{
  "name": "<tool>",
  "version": "<version>",
  "description": "<description>",
  "main": "index.js",
  "bin": {
    "<tool>": "index.js"
  },
  "scripts": {
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "author": "<author> <<email>>",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "<repository_url>"
  },
  "os": ["darwin", "linux", "win32"],
  "cpu": ["x64", "arm64"],
  "files": [
    "bin/**/*",
    "index.js"
  ]
}
```

#### index.js

```js
#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

// Map platform and architecture to binary path
function getBinaryPath() {
  const platform = os.platform();
  const arch = os.arch();
  
  let binaryName = '<tool>';
  let platformDir = '';
  
  if (platform === 'darwin') {
    if (arch === 'x64') {
      platformDir = 'darwin_amd64';
    } else if (arch === 'arm64') {
      platformDir = 'darwin_arm64';
    }
  } else if (platform === 'linux') {
    if (arch === 'x64') {
      platformDir = 'linux_amd64';
    } else if (arch === 'arm64') {
      platformDir = 'linux_arm64';
    }
  } else if (platform === 'win32') {
    platformDir = 'windows_amd64';
    binaryName += '.exe';
  }
  
  if (!platformDir) {
    console.error(`Unsupported platform: ${platform}_${arch}`);
    process.exit(1);
  }
  
  return path.join(__dirname, 'bin', platformDir, binaryName);
}

// Get path to the binary
const binaryPath = getBinaryPath();

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  console.error(`Binary not found: ${binaryPath}`);
  process.exit(1);
}

// Make sure the binary is executable
try {
  fs.chmodSync(binaryPath, 0o755);
} catch (err) {
  console.error('Error making binary executable:', err);
  process.exit(1);
}

// Execute the binary with the same arguments
const proc = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit'
});

// Handle process exit
proc.on('exit', (code) => {
  process.exit(code || 0);
});

// Handle errors
proc.on('error', (err) => {
  console.error('Error executing <tool>:', err);
  process.exit(1);
});
```

## Best Practices

1. **Binary Size**: Go binaries can be large. Consider using compression techniques like UPX to reduce binary size.
2. **Platform Detection**: Ensure accurate platform detection logic in wrapper scripts.
3. **Error Handling**: Provide clear error messages when a platform is not supported.
4. **Versioning**: Follow semantic versioning for all packages.
5. **Documentation**: Include usage documentation in the package description.

## Future Package Formats

### Homebrew Formula

```ruby
class Tool < Formula
  desc "<description>"
  homepage "<homepage>"
  url "<url-to-tarball>"
  sha256 "<checksum>"
  version "<version>"
  
  def install
    bin.install "tool"
  end
  
  test do
    system "#{bin}/tool", "--version"
  end
end
```

### Debian Package

Structure and specifications for .deb packages.

### RPM Package

Structure and specifications for .rpm packages.