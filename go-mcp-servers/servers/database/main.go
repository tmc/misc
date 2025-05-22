package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

func main() {
	server := mcpframework.NewServer("database-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides database operations including SQL queries, schema inspection, and data management for SQLite databases.")

	// Register database tools
	registerDatabaseTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerDatabaseTools(server *mcpframework.Server) {
	// Execute SQL query tool
	server.RegisterTool("sql_query", "Execute a SQL query", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "SQL query to execute",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rows to return for SELECT queries",
				"default":     100,
			},
		},
		Required: []string{"database_path", "query"},
	}, handleSQLQuery)

	// List tables tool
	server.RegisterTool("list_tables", "List all tables in the database", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
		},
		Required: []string{"database_path"},
	}, handleListTables)

	// Describe table tool
	server.RegisterTool("describe_table", "Get the schema of a table", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
			"table_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the table to describe",
			},
		},
		Required: []string{"database_path", "table_name"},
	}, handleDescribeTable)

	// Create database tool
	server.RegisterTool("create_database", "Create a new SQLite database", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path for the new database file",
			},
		},
		Required: []string{"database_path"},
	}, handleCreateDatabase)

	// Execute SQL script tool
	server.RegisterTool("execute_script", "Execute a SQL script file", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
			"script_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQL script file",
			},
		},
		Required: []string{"database_path", "script_path"},
	}, handleExecuteScript)

	// Export table tool
	server.RegisterTool("export_table", "Export table data to CSV", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
			"table_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the table to export",
			},
			"output_path": map[string]interface{}{
				"type":        "string",
				"description": "Output CSV file path",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rows to export",
				"default":     1000,
			},
		},
		Required: []string{"database_path", "table_name", "output_path"},
	}, handleExportTable)

	// Database info tool
	server.RegisterTool("database_info", "Get database information and statistics", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"database_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the SQLite database file",
			},
		},
		Required: []string{"database_path"},
	}, handleDatabaseInfo)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing for database files
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		var resources []mcpframework.Resource
		
		// Look for SQLite databases in current directory
		matches, err := filepath.Glob("*.db")
		if err != nil {
			return &mcpframework.ListResourcesResult{Resources: resources}, nil
		}

		for _, match := range matches {
			resources = append(resources, mcpframework.Resource{
				URI:         "sqlite://" + match,
				Name:        match,
				Description: "SQLite database file",
				MimeType:    "application/x-sqlite3",
			})
		}

		// Also look for .sqlite files
		sqliteMatches, err := filepath.Glob("*.sqlite")
		if err == nil {
			for _, match := range sqliteMatches {
				resources = append(resources, mcpframework.Resource{
					URI:         "sqlite://" + match,
					Name:        match,
					Description: "SQLite database file",
					MimeType:    "application/x-sqlite3",
				})
			}
		}

		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading for database content
	server.RegisterResourceHandler("sqlite://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		dbPath := strings.TrimPrefix(uri, "sqlite://")
		return getDatabaseResourceContent(dbPath)
	})
}

func getDatabaseResourceContent(dbPath string) (*mcpframework.ReadResourceResult, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get basic database info
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Database: %s\n\n", dbPath))

	// List tables
	tables, err := getTables(db)
	if err != nil {
		return nil, err
	}

	result.WriteString("Tables:\n")
	for _, table := range tables {
		result.WriteString(fmt.Sprintf("- %s\n", table))
	}

	return &mcpframework.ReadResourceResult{
		Contents: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func handleSQLQuery(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
		Query        string `json:"query"`
		Limit        int    `json:"limit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Limit == 0 {
		args.Limit = 100
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	// Determine if this is a SELECT query
	query := strings.TrimSpace(strings.ToUpper(args.Query))
	isSelect := strings.HasPrefix(query, "SELECT")

	if isSelect {
		return executeSelectQuery(db, args.Query, args.Limit), nil
	} else {
		return executeNonSelectQuery(db, args.Query), nil
	}
}

func executeSelectQuery(db *sql.DB, query string, limit int) *mcpframework.CallToolResult {
	// Add LIMIT if not present
	if limit > 0 && !strings.Contains(strings.ToUpper(query), "LIMIT") {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Query failed: %s", err.Error()),
				},
			},
			IsError: true,
		}
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to get columns: %s", err.Error()),
				},
			},
			IsError: true,
		}
	}

	var result strings.Builder
	
	// Header
	result.WriteString(strings.Join(columns, " | "))
	result.WriteString("\n")
	result.WriteString(strings.Repeat("-", len(strings.Join(columns, " | "))))
	result.WriteString("\n")

	// Data rows
	rowCount := 0
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to scan row: %s", err.Error()),
					},
				},
				IsError: true,
			}
		}

		var rowValues []string
		for _, v := range values {
			if v == nil {
				rowValues = append(rowValues, "NULL")
			} else {
				rowValues = append(rowValues, fmt.Sprintf("%v", v))
			}
		}

		result.WriteString(strings.Join(rowValues, " | "))
		result.WriteString("\n")
		rowCount++
	}

	result.WriteString(fmt.Sprintf("\n(%d rows)", rowCount))

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}
}

func executeNonSelectQuery(db *sql.DB, query string) *mcpframework.CallToolResult {
	result, err := db.Exec(query)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Query failed: %s", err.Error()),
				},
			},
			IsError: true,
		}
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertId, _ := result.LastInsertId()

	var response strings.Builder
	response.WriteString("Query executed successfully\n")
	response.WriteString(fmt.Sprintf("Rows affected: %d\n", rowsAffected))
	if lastInsertId > 0 {
		response.WriteString(fmt.Sprintf("Last insert ID: %d\n", lastInsertId))
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: response.String(),
			},
		},
	}
}

func handleListTables(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	tables, err := getTables(db)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to list tables: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: strings.Join(tables, "\n"),
			},
		},
	}, nil
}

func handleDescribeTable(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
		TableName    string `json:"table_name"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	query := fmt.Sprintf("PRAGMA table_info(%s)", args.TableName)
	rows, err := db.Query(query)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to get table info: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer rows.Close()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Table: %s\n\n", args.TableName))
	result.WriteString("Column | Type | Not Null | Default | Primary Key\n")
	result.WriteString("-------|------|----------|---------|------------\n")

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		if err != nil {
			continue
		}

		def := "NULL"
		if defaultValue.Valid {
			def = defaultValue.String
		}

		result.WriteString(fmt.Sprintf("%s | %s | %v | %s | %v\n",
			name, dataType, notNull == 1, def, pk == 1))
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func handleCreateDatabase(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to verify database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Database created successfully: %s", args.DatabasePath),
			},
		},
	}, nil
}

func handleExecuteScript(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
		ScriptPath   string `json:"script_path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Read script file
	scriptContent, err := os.ReadFile(args.ScriptPath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to read script file: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	// Execute script
	_, err = db.Exec(string(scriptContent))
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Script execution failed: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Script executed successfully: %s", args.ScriptPath),
			},
		},
	}, nil
}

func handleExportTable(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
		TableName    string `json:"table_name"`
		OutputPath   string `json:"output_path"`
		Limit        int    `json:"limit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Limit == 0 {
		args.Limit = 1000
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", args.TableName, args.Limit)
	rows, err := db.Query(query)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Query failed: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer rows.Close()

	// Create output file
	file, err := os.Create(args.OutputPath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create output file: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer file.Close()

	// Get column names and write header
	columns, err := rows.Columns()
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to get columns: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	// Write CSV header
	fmt.Fprintf(file, "%s\n", strings.Join(columns, ","))

	// Write data rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			continue
		}

		var csvValues []string
		for _, v := range values {
			if v == nil {
				csvValues = append(csvValues, "")
			} else {
				csvValues = append(csvValues, fmt.Sprintf("%v", v))
			}
		}

		fmt.Fprintf(file, "%s\n", strings.Join(csvValues, ","))
		rowCount++
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Exported %d rows to %s", rowCount, args.OutputPath),
			},
		},
	}, nil
}

func handleDatabaseInfo(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		DatabasePath string `json:"database_path"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	db, err := sql.Open("sqlite3", args.DatabasePath)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to open database: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}
	defer db.Close()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Database: %s\n\n", args.DatabasePath))

	// Get file info
	if info, err := os.Stat(args.DatabasePath); err == nil {
		result.WriteString(fmt.Sprintf("File size: %d bytes\n", info.Size()))
		result.WriteString(fmt.Sprintf("Modified: %s\n\n", info.ModTime().Format("2006-01-02 15:04:05")))
	}

	// Get SQLite version
	var version string
	err = db.QueryRow("SELECT sqlite_version()").Scan(&version)
	if err == nil {
		result.WriteString(fmt.Sprintf("SQLite version: %s\n\n", version))
	}

	// Get table count and info
	tables, err := getTables(db)
	if err == nil {
		result.WriteString(fmt.Sprintf("Tables: %d\n", len(tables)))
		for _, table := range tables {
			var count int
			err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
			if err == nil {
				result.WriteString(fmt.Sprintf("  %s: %d rows\n", table, count))
			}
		}
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func getTables(db *sql.DB) ([]string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}