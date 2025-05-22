# Testing the Protoc-Gen-Anything Web Playground

## Running the Project

First, let's build the WebAssembly module:

```bash
# Navigate to the wasm directory
cd /Volumes/tmc/go/src/github.com/tmc/misc/protogen-web-playground/wasm

# Build the WebAssembly binary
make build
```

Then start the frontend application:

```bash
# Navigate to the frontend directory
cd /Volumes/tmc/go/src/github.com/tmc/misc/protogen-web-playground/frontend

# Install dependencies (first time only)
npm install

# Start the development server
npm start
```

This will start the application at http://localhost:3000

## Test Cases

### 1. Basic Generation

1. Open the web playground in your browser
2. The default proto file and template should be loaded
3. Verify that output is generated in the right panel
4. Make a small change to the proto file (e.g., change a field name)
5. The output should update automatically after a short delay (real-time mode)

### 2. Manual Mode

1. Click the pause button in the output panel
2. This switches to manual mode
3. Make a change to the proto file or template
4. Verify that the output doesn't update automatically
5. Click the generate button (circular arrow)
6. Verify that output is updated based on the new input

### 3. GitHub Integration

**Note**: This requires a GitHub personal access token with gist permissions.

1. Click the "Gists" button in the header
2. Click the "Login" button and enter your GitHub token
3. Click "Save" to create a new gist
4. Enter a description and click "Save"
5. Verify that the gist is created and the URL is updated
6. Try loading the gist by refreshing the page

### 4. Sharing Via URL

1. Make some changes to the proto file and template
2. Click the "Share" button in the header
3. Copy the generated URL
4. Open the URL in a new browser tab
5. Verify that the proto file and template are loaded from the URL

### 5. Settings

1. Click the "Settings" button in the header
2. Change some options (e.g., turn off "Continue on errors")
3. Verify that changes are applied to the generation
4. Try switching between real-time and manual modes
5. Verify that the mode changes work as expected

## Browser Compatibility

The web playground should work in modern browsers that support WebAssembly:

- Chrome/Edge (recent versions)
- Firefox (recent versions)
- Safari (recent versions)

## Troubleshooting

If you encounter issues:

1. **WebAssembly Load Error**: Check browser console for errors. Ensure WASM file is properly built.
2. **Real-time Updates Not Working**: Ensure you're in real-time mode (play button should be visible).
3. **GitHub API Errors**: Check that your token has the right permissions (gist scope).
4. **Empty Output**: Check console for JavaScript errors, may indicate template processing issue.

## Known Limitations

- Full protobuf compiler features are not all supported in the WebAssembly version
- Large proto files may cause performance issues in the browser
- GitHub token is stored in local storage - log out when done on shared computers