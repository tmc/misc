# ant-proxy

A proxy server that accepts Anthropic API requests and routes them to other LLM providers (Google Gemini, OpenAI, or Ollama).

## Features

- Intercepts Anthropic API calls
- Routes requests to other LLM providers, transforming request/response formats as needed
- Supports providers: Gemini (implemented), OpenAI and Ollama (planned)
- Records sessions for later analysis
- Configurable via environment variables and command-line flags

## Usage

### Requirements

- Go 1.24 or later
- Anthropic API key (for authentication validation)
- Gemini API key (for routing to Gemini)

### Configuration

Set the following environment variables:

```bash
export ANTHROPIC_API_KEY=your_anthropic_key
export GOOGLE_API_KEY=your_gemini_key
```

### Running the proxy

```bash
# Basic usage
go run . -listen-address :9091

# Run with web server mode
go run . -w -port 8072

# Run with verbose logging
go run . -v
```

### Directing clients to use the proxy

Configure your Anthropic client to use the proxy by setting the base URL:

```
ANTHROPIC_API_BASE=http://localhost:9091
```

## Development

Build the project:

```bash
go build
```

Test the proxy:

```bash
# Using curl
curl -X POST http://localhost:9091/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-7-sonnet-20250219",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "Hello, world!"
      }
    ]
  }'
```

## License

Copyright © 2025