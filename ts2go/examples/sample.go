// Code generated from examples/sample.ts; DO NOT EDIT.
package sample


import (
	"encoding/json"

)


// Sample TypeScript schema for testing ts2go
const VERSION = "1.0.0"

const DEBUG_MODE = true


// User role in the system.
// Can be admin, user, or guest.
type UserRole string


// Base interface for all API responses
type ApiResponse struct {
	// Response status code
	StatusCode float64 `json:"status_code"`
	// Response message
	Message string `json:"message"`
	// Success indicator
	Success bool `json:"success"`
}
// User information.
// Contains basic user data.
type User struct {
	// Unique user ID
	ID string `json:"id"`
	// User's full name
	Name string `json:"name"`
	// User's email address
	Email string `json:"email"`
	// User's role in the system
	Role UserRole `json:"role"`
	// Optional profile picture URL
	ProfilePicURL string `json:"profile_pic_url,omitempty"`
	// User's creation date
	CreatedAt string `json:"created_at"`
	// Whether the user account is active
	IsActive bool `json:"is_active"`
	// User's metadata, can contain any additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// List of user's tags
	Tags []string `json:"tags,omitempty"`
}
// Paginated response with users
type UserListResponse struct {
	// Total number of users
	TotalCount float64 `json:"total_count"`
	// Page number
	Page float64 `json:"page"`
	// Number of items per page
	PerPage float64 `json:"per_page"`
	// List of users
	Users []User `json:"users"`
	// Optional next cursor for pagination
	NextCursor string `json:"next_cursor,omitempty"`
}
// Content that can be different types
type Content struct {
	// Type of content
	Type string `json:"type"`
	// Text content if type is text
	Text string `json:"text,omitempty"`
	// URL if type is image or file
	URL string `json:"url,omitempty"`
	// MIME type for the content
	MimeType string `json:"mime_type,omitempty"`
}
// Request to create a new user
type CreateUserRequest struct {
	// User's name
	Name string `json:"name"`
	// User's email
	Email string `json:"email"`
	// Optional user role, defaults to "user"
	Role *UserRole `json:"role,omitempty"`
	// Optional profile picture as content
	ProfilePicture *Content `json:"profile_picture,omitempty"`
}
