// Code generated from examples/simple.ts; DO NOT EDIT.
package example


// Common imports
import (
	"encoding/json"
)


// Version of the API
const API_VERSION = "1.0.0"
// Maximum number of API requests per minute
const MAX_REQUESTS_PER_MINUTE = 100
// Whether debug mode is enabled
const DEBUG_MODE = false


// ID A basic type alias for strings
type ID string
// OrderStatus Status values for an order
type OrderStatus string


// Point Represents a point in 2D space
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
// Rectangle Represents a rectangle defined by two points
type Rectangle struct {
	// Top-left point of the rectangle
	TopLeft Point `json:"topLeft"`
	// Bottom-right point of the rectangle
	BottomRight Point `json:"bottomRight"`
}
// User Represents a user in the system
type User struct {
	// Unique identifier for the user
	ID ID `json:"id"`
	// User's full name
	Name string `json:"name"`
	// User's email address (optional)
	Email string `json:"email,omitempty"`
	// User's age
	Age float64 `json:"age"`
	// Array of role names
	Roles []string `json:"roles"`
}
