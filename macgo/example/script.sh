#!/bin/bash
# This script creates a sample app bundle for the example

# Get the script's directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Build the example binary
echo "Building example binary..."
go build -o example main.go

# Create an app bundle directory structure
echo "Creating app bundle structure..."
mkdir -p Example.app/Contents/MacOS
mkdir -p Example.app/Contents/Resources

# Create Info.plist
cat > Example.app/Contents/Info.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>example</string>
    <key>CFBundleIdentifier</key>
    <string>com.example.macgo-example</string>
    <key>CFBundleName</key>
    <string>Example</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# Copy the example binary to the app bundle
cp example Example.app/Contents/MacOS/

# Make it executable
chmod +x Example.app/Contents/MacOS/example

echo "App bundle created: $SCRIPT_DIR/Example.app"
echo ""
echo "Build with:"
echo "cd $SCRIPT_DIR && go build -o example main.go"
echo ""
echo "Run with verbose output:"
echo "cd $SCRIPT_DIR && ./example --macgo-verbose"
echo ""
echo "Note: Using 'go run' won't work properly since macgo requires a built binary"