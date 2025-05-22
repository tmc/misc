# SQL DDL Example

This example demonstrates how to generate SQL DDL scripts and Go models from Protocol Buffer definitions using `protoc-gen-anything`.

## Features

- Converts protobuf messages to SQL tables
- Maps protobuf field types to SQL column types
- Supports primary keys, foreign keys, and indexes
- Handles table relationships (one-to-one, one-to-many)
- Generates GORM-compatible Go models
- Supports custom SQL options for fine-grained control

## Usage

Generate the SQL DDL and Go models with:

```bash
make generate
```

This will produce:
- `schema.sql` - SQL schema creation script
- `models.go` - Go models for ORM access

## SQL Options

The example uses custom protocol buffer options to control SQL generation:

### Message Options
- `table_name` - Override the SQL table name
- `skip` - Skip this message when generating SQL
- `comment` - Add a table comment
- `schema` - Set the database schema
- `engine` - Set the storage engine (e.g., "InnoDB")
- `charset` - Set the character set
- `collation` - Set the collation
- `is_table` - Explicitly mark this message as a table

### Field Options
- `column_name` - Override the SQL column name
- `column_type` - Set the SQL column type
- `constraints` - Add SQL constraints (e.g., "NOT NULL", "UNIQUE")
- `default_value` - Set a default value
- `skip` - Skip this field when generating SQL
- `primary_key` - Mark this field as a primary key
- `foreign_key` - Define a foreign key relationship
- `index` - Create an index on this field

## Type Mappings

The example demonstrates mapping between Protocol Buffer and SQL types:

| Protocol Buffer Type | SQL Type           | Go Type          |
| -------------------- | ------------------ | ---------------- |
| int32, sint32        | INT                | int32            |
| int64, sint64        | BIGINT             | int64            |
| uint32               | INT UNSIGNED       | uint32           |
| uint64               | BIGINT UNSIGNED    | uint64           |
| float                | FLOAT              | float32          |
| double               | DOUBLE             | float64          |
| bool                 | BOOLEAN            | bool             |
| string               | VARCHAR(255)       | string           |
| bytes                | BLOB               | []byte           |
| enum                 | VARCHAR(50)        | Enum Type        |
| Timestamp            | TIMESTAMP          | time.Time        |
| repeated             | JSON               | []T              |
| map                  | JSON               | map[string]T     |

## Integration

The generated Go models can be used with GORM:

```go
package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	
	"myproject/gen/models"
)

func main() {
	dsn := "user:password@tcp(127.0.0.1:3306)/blog_db?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database")
	}
	
	// Create a new user
	user := &models.User{
		ID:    "user-123",
		Name:  "John Doe",
		Email: "john@example.com",
		Role:  models.USER_ROLE_USER,
	}
	
	// Save to database
	db.Create(user)
	
	// Create a post by this user
	post := &models.Post{
		ID:       "post-456",
		AuthorID: user.ID,
		Title:    "My First Post",
		Content:  "Hello, world!",
		Status:   models.POST_STATUS_PUBLISHED,
	}
	
	// Save to database
	db.Create(post)
}
```