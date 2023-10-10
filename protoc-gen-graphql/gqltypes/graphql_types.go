package gqltypes

// Schema represents a GraphQL schema.
type Schema struct {
	Types   []*Type
	Inputs  []*Input
	Enums   []*Enum
	Scalars []string
	Unions  []*Union
	// Extensions []*Extension

	RootQuery    *Type
	RootMutation *Type
}

// Input represents an input type for a GraphQL query.
type Input struct {
	Name    string
	Comment string
	Fields  []*Field
}

// String returns the string representation of the input type.
func (i *Input) String() string {
	return i.Name
}

// Field represents a field in a GraphQL type.
type Field struct {
	Name    string
	Comment string
	Inputs  []*Input
	// Output     *Type
	Type       string
	Directives []*Directive
}

// Input represents an input type for a GraphQL query.
func (f Field) Input() *Input {
	if len(f.Inputs) == 0 {
		return nil
	}
	return f.Inputs[0]
}

// Type represents a GraphQL type.
type Type struct {
	Name       string
	IsRequired bool
	Comment    string
	Fields     []*Field
	Directives []*Directive
}

// String returns the string representation of the type.
func (t *Type) String() string {
	if t.IsRequired {
		return t.Name + "!"
	}
	return t.Name
}

// Directive represents a GraphQL directive.
type Directive struct {
	Name    string
	Comment string
	Fields  []*Field
}

// Union represents a GraphQL union.
type Union struct {
	Name  string
	Types []string
}

// Enum represents a GraphQL enum.
type Enum struct {
	Name    string
	Comment string
	Options []*EnumOption
}

// EnumOption represents an option in a GraphQL enum.
type EnumOption struct {
	Name    string
	Comment string
}

// Mutation represents a GraphQL mutation.
type Mutation struct {
	Name       string
	Comment    string
	Input      *Input
	Output     *Type
	Directives []*Directive
}

// Extension represents a GraphQL schema extension.
type Extension struct {
	Name       string
	Comment    string
	Extends    string
	Input      *Input
	Output     *Type
	Directives []*Directive
}
