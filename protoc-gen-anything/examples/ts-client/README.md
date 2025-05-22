# TypeScript Client Example

This example demonstrates how to generate a zero-dependency TypeScript client from Protocol Buffer definitions using `protoc-gen-anything`.

## Features

- Generates TypeScript interfaces for all Protocol Buffer messages
- Creates a strongly-typed client class for each service
- Maps Protocol Buffer types to TypeScript types
- Extracts HTTP method and path information from `google.api.http` annotations
- Generates a convenient API for making HTTP requests
- Uses native `fetch` API with no external dependencies

## Usage

Generate the TypeScript client with:

```bash
make generate
```

This will produce TypeScript files in the `gen` directory:
- `HTTPService.client.ts` - The client class for the HTTP service
- `index.ts` - An index file that exports all clients

## How It Works

The example uses service-scoped templates that:

1. Generate TypeScript interfaces for request and response messages
2. Create client classes for each service
3. Map HTTP paths using `google.api.http` annotations
4. Include helper methods for making API requests

## Template Breakdown

The template uses several helper functions:

- `tsType` - Maps protobuf types to TypeScript types
- `methodExtension` - Extracts HTTP method and path information
- `extractPathParams` - Extracts path parameters from URL templates

The generated client supports:
- Authentication via API key
- Custom headers
- Error handling
- JSON request/response parsing
- Path parameter interpolation

## Integration

The generated client can be used in any TypeScript/JavaScript project:

```typescript
import { HTTPServiceClient } from './gen';

const client = new HTTPServiceClient('https://api.example.com', 'your-api-key');

// Call methods directly
const response = await client.getMessage('message-id');
console.log(response.data);
```