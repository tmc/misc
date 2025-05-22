# Running and Testing the Protoc-Gen-Anything Web Playground

## Quick Test with Simple HTML Page

You can quickly test that the WebAssembly module is working properly:

```bash
# Navigate to the project directory
cd /Volumes/tmc/go/src/github.com/tmc/misc/protogen-web-playground

# Start a simple HTTP server
python3 -m http.server 8000 --directory frontend/public
```

Then open your browser to http://localhost:8000/test.html to test the WebAssembly functionality.

## Running the Full Application

To run the full React application:

```bash
# Navigate to the frontend directory
cd /Volumes/tmc/go/src/github.com/tmc/misc/protogen-web-playground/frontend

# Install dependencies
npm install --legacy-peer-deps

# Start the development server
npm start
```

This will start the React application at http://localhost:3000

## What to Test

1. **Basic Generation**: Edit the protocol buffer schema and template in the left and middle panels. The output should automatically update in the right panel.

2. **Real-time vs. Manual Mode**: Click the play/pause button in the top-right corner of the output panel to toggle between real-time and manual generation modes.

3. **GitHub Integration**: Click the "Gists" button to open the Gist panel. You can save your configuration to a GitHub Gist or load from an existing one (requires a GitHub token).

4. **Sharing**: Use the "Share" button to generate a shareable URL with your current configuration embedded.

5. **Settings**: Click the "Settings" button to access generation options like continuing on errors or enabling verbose output.

## Troubleshooting

If you encounter any issues:

1. **WebAssembly Not Loading**: Check your browser console for errors. Make sure the wasm_exec.js file was copied correctly to the frontend/public/wasm directory.

2. **React Not Starting**: Make sure you're using the --legacy-peer-deps flag when installing packages due to dependency conflicts.

3. **Generation Errors**: Check your browser console for detailed error messages. Make sure your protocol buffer schema is valid.

4. **GitHub API Issues**: If you have trouble with GitHub integration, check that your token has the necessary scope for creating and reading Gists.