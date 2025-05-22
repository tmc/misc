# Protoc-Gen-Anything Web Playground Architecture

## Overview

The protoc-gen-anything web playground is a browser-based tool that allows users to interactively:
1. Define Protocol Buffer schemas
2. Create and edit Go templates
3. Generate output in real-time
4. Load and save templates from/to GitHub Gists
5. Share playground configurations via URLs

## System Components

### Frontend

- **React-based SPA**: A single-page application with a multi-panel interface
- **Monaco Editor**: Code editors with syntax highlighting for Protobuf and Go templates
- **Split-Pane Layout**: Resizable panels for editing proto files, templates, and viewing output
- **GitHub Integration**: Authenticate and interact with GitHub API for Gist operations

### Backend

- **Go API Server**: Handles requests from the frontend
- **Protocol Buffer Compiler Bridge**: Executes protoc with protoc-gen-anything
- **Template Processing**: Manages template loading, validation, and execution
- **Real-time Processing**: WebSocket connection for live updates as users type
- **GitHub API Integration**: Proxy for GitHub operations to avoid CORS issues

### Workflow

1. User edits Protobuf schema in the left panel
2. User creates/edits Go templates in the middle panel
3. System compiles Protobuf and runs protoc-gen-anything with the templates
4. Generated output appears in real-time in the right panel
5. User can save/load configurations to/from GitHub Gists
6. Share links generate URLs with embedded or referenced configurations

## API Endpoints

### Backend REST API

- `POST /api/generate`: Process proto files and templates, return generated output
- `GET /api/templates/examples`: Get example templates
- `POST /api/templates/validate`: Validate a template
- `GET /api/github/gists/{id}`: Fetch a GitHub Gist
- `POST /api/github/gists`: Create a new GitHub Gist
- `PUT /api/github/gists/{id}`: Update an existing GitHub Gist

### WebSocket API

- `/ws/session`: Real-time connection for ongoing edits and generation
  - Client sends: Proto file or template updates
  - Server responds: Updated generation results

## Data Models

### Configuration

```json
{
  "id": "unique-session-id",
  "proto": {
    "files": [
      {
        "name": "example.proto",
        "content": "syntax = \"proto3\";\n..."
      }
    ]
  },
  "templates": [
    {
      "name": "{{.Service.GoName}}_service.go.tmpl",
      "content": "package {{.File.GoPackageName}}\n..."
    }
  ],
  "options": {
    "includeImports": true,
    "outputFormat": "zip",
    "continueOnError": true
  }
}
```

### Generation Result

```json
{
  "success": true,
  "files": [
    {
      "name": "example_service.go",
      "content": "package example\n..."
    }
  ],
  "logs": [
    {"level": "info", "message": "Generating file: example_service.go"}
  ],
  "errors": []
}
```

## Deployment Architecture

- **Frontend**: Static files served from CDN
- **Backend**: Containerized Go service
- **State Management**: Stateless design with temporary file storage
- **Caching**: Redis for template and generation caching
- **Security**: Rate limiting, input validation, and sanitization

## GitHub/Gist Integration

- **Authentication**: GitHub OAuth for creating/updating Gists
- **Anonymous Usage**: Read-only access to public Gists without authentication
- **Gist Structure**:
  - One file for configuration (`playground.json`)
  - Separate files for each proto file and template
  - README.md with instructions and playground link

## Real-time Processing Implementation

1. Use debounced updates to prevent excessive generation
2. Implement incremental compilation for large proto files
3. Cache previous generations to improve response time
4. Stream large outputs using WebSockets
5. Provide visual feedback during processing