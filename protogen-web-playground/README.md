# Protoc-Gen-Anything Web Playground

An interactive web-based playground for the versatile [protoc-gen-anything](https://github.com/tmc/misc/tree/master/protoc-gen-anything) plugin that generates anything from protobuf files.

## Features

- Browser-based editor for Protocol Buffer schemas
- Interactive Go template editor with syntax highlighting
- Real-time generation preview using WebAssembly
- GitHub/Gist integration for loading and saving templates
- Configurable template options and settings
- Shareable playground configurations via URLs

## Development

### Prerequisites

- Go 1.20+
- [Bun](https://bun.sh/) 1.0+ (Fast JavaScript runtime and package manager)
- Protocol Buffers compiler (protoc)
- Docker (optional, for containerized development)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/tmc/misc.git
cd misc/protogen-web-playground

# Install backend dependencies
go mod tidy

# Install frontend dependencies
cd frontend
bun install
cd ..

# Run the development server
make dev
```

### Project Structure

```
protogen-web-playground/
├── api/               # Backend API handlers
├── cmd/               # Command-line entrypoints
├── frontend/          # React frontend application
│   ├── public/        # Static files
│   └── src/           # React components and logic
├── pkg/               # Shared Go packages
│   ├── compiler/      # Protobuf compilation logic
│   ├── generator/     # Template generation
│   ├── github/        # GitHub API integration
│   └── websocket/     # Real-time communication
├── templates/         # Example templates
├── wasm/              # WebAssembly implementation
└── proto/             # Example proto files
```

## WebAssembly Implementation

This playground uses WebAssembly to run the protoc-gen-anything generator directly in the browser. This provides several benefits:

1. **Improved Privacy**: All processing happens client-side; no server needed for template generation
2. **Better Performance**: No network latency for generations
3. **Offline Support**: Works without an internet connection after initial load

The WebAssembly module is built from Go code in the `wasm/` directory.

## Usage

1. Open the playground in your browser
2. Write or paste your Protocol Buffer schema in the left editor
3. Create or modify Go templates in the middle editor
4. View the generated output in real-time in the right panel
5. Save your work to a GitHub Gist or share via URL

## GitHub/Gist Integration

The playground can load templates directly from GitHub Gists. To use this feature:

1. Create a Gist with your proto files and templates
2. Use the "Load from Gist" button and enter the Gist ID
3. Alternatively, append `?gist=GIST_ID` to the playground URL

To save your work to a Gist:

1. Click the "Save to Gist" button
2. Authenticate with GitHub if prompted
3. Provide a description for your Gist
4. Choose public or private visibility

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [protoc-gen-anything](https://github.com/tmc/misc/tree/master/protoc-gen-anything) - The core plugin that drives this playground
- [Monaco Editor](https://microsoft.github.io/monaco-editor/) - The powerful code editor used in the playground
- [Protocol Buffers](https://developers.google.com/protocol-buffers) - Google's language-neutral, platform-neutral extensible mechanism for serializing structured data