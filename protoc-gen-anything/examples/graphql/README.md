# GraphQL Schema Example

This example demonstrates how to generate GraphQL schemas and resolvers from Protocol Buffer definitions using `protoc-gen-anything`.

## Features

- Converts protobuf messages to GraphQL types
- Handles nested message structures
- Supports GraphQL interfaces and unions (via message options)
- Generates properly typed TypeScript resolvers
- Maps protobuf enums to GraphQL enums
- Supports custom field options for GraphQL-specific features

## Usage

Generate the GraphQL schema and resolvers with:

```bash
make generate
```

This will produce:
- `schema.graphql` - The GraphQL schema
- `resolvers.ts` - TypeScript resolver implementations

## Schema Options

The example uses custom protocol buffer options to control GraphQL schema generation:

### Message Options
- `skip` - Skip this message when generating GraphQL schema
- `name` - Override the GraphQL type name
- `description` - Set the GraphQL type description 
- `input_type` - Treat this message as a GraphQL input type
- `interface` - Treat this message as a GraphQL interface

### Field Options
- `skip` - Skip this field when generating GraphQL schema
- `name` - Override the GraphQL field name
- `description` - Set the GraphQL field description
- `deprecated` - Mark this field as deprecated with a reason

## Type Mappings

The example demonstrates mapping between Protocol Buffer and GraphQL types:

| Protocol Buffer Type | GraphQL Type |
| -------------------- | ------------ |
| string               | String       |
| int32, int64         | Int          |
| float, double        | Float        |
| bool                 | Boolean      |
| Timestamp            | DateTime     |
| Struct, Any          | JSON         |
| Enum                 | Enum         |
| Message              | Object       |
| repeated             | List         |
| oneof                | Union        |

## Integration

The generated GraphQL schema and resolvers can be used with any GraphQL server implementation:

```typescript
import { ApolloServer } from 'apollo-server';
import { typeDefs } from './gen/schema';
import { resolvers } from './gen/resolvers';
import { BlogService } from './services/blog.service';

const server = new ApolloServer({
  typeDefs,
  resolvers,
  context: {
    blogService: new BlogService()
  }
});

server.listen().then(({ url }) => {
  console.log(`GraphQL server running at ${url}`);
});
```