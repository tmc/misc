// Default proto file content
export const DEFAULT_PROTO = `syntax = "proto3";

package example;

message Example {
  string id = 1;
  string name = 2;
  
  message NestedExample {
    int32 value = 1;
  }
}

service ExampleService {
  rpc GetExample(GetExampleRequest) returns (GetExampleResponse);
}

message GetExampleRequest {
  string id = 1;
}

message GetExampleResponse {
  Example example = 1;
}`;

// Default template content
export const DEFAULT_TEMPLATE = `package {{.File.GoPackageName}}

// Adds Foobar() method to {{.Message.GoIdent}}
func (m *{{.Message.GoIdent.GoName}}) Foobar() {
  // Implementation goes here
}`;

// Proto language definition for syntax highlighting
export const PROTO_LANGUAGE = {
  id: 'proto',
  extensions: ['.proto'],
  aliases: ['Protocol Buffers', 'protobuf'],
  mimetypes: ['text/x-protobuf'],
  
  // Tokens definition for syntax highlighting
  tokenizer: {
    root: [
      [/syntax|package|option|import/, 'keyword'],
      [/message|service|enum|rpc|returns/, 'keyword.declaration'],
      [/\b(double|float|int32|int64|uint32|uint64|sint32|sint64|fixed32|fixed64|sfixed32|sfixed64|bool|string|bytes)\b/, 'type'],
      [/\b(true|false)\b/, 'keyword.constant'],
      [/\b\d+\b/, 'number'],
      [/".*?"/, 'string'],
      [/\/\/.*$/, 'comment'],
      [/\/\*/, { token: 'comment.block', next: '@comment' }],
    ],
    comment: [
      [/[^/*]+/, 'comment.block'],
      [/\*\//, { token: 'comment.block', next: '@pop' }],
      [/[/*]/, 'comment.block']
    ]
  }
};