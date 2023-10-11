package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"github.com/pgvector/pgvector-go"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("age"),
		field.String("description").Optional(),
		field.Other("embedding", pgvector.Vector{}).SchemaType(map[string]string{
			dialect.Postgres: "vector(1536)",
		}).Optional().Sensitive(), // we mark the field as sensitive, so it won't be fetched by default.
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return nil
}
