/**
 * Sample TypeScript schema for testing ts2go
 */

export const VERSION = "1.0.0";
export const DEBUG_MODE = true;

/**
 * User role in the system.
 * Can be admin, user, or guest.
 */
export type UserRole = "admin" | "user" | "guest";

/**
 * Base interface for all API responses
 */
export interface ApiResponse {
  /** Response status code */
  status_code: number;
  /** Response message */
  message: string;
  /** Success indicator */
  success: boolean;
}

/**
 * User information.
 * Contains basic user data.
 */
export interface User {
  /** Unique user ID */
  id: string;
  /** User's full name */
  name: string;
  /** User's email address */
  email: string;
  /** User's role in the system */
  role: UserRole;
  /** Optional profile picture URL */
  profile_pic_url?: string;
  /** User's creation date */
  created_at: string;
  /** Whether the user account is active */
  is_active: boolean;
  /** User's metadata, can contain any additional information */
  metadata?: Record<string, any>;
  /** List of user's tags */
  tags?: string[];
}

/**
 * Paginated response with users
 */
export interface UserListResponse extends ApiResponse {
  /** Total number of users */
  total_count: number;
  /** Page number */
  page: number;
  /** Number of items per page */
  per_page: number;
  /** List of users */
  users: User[];
  /** Optional next cursor for pagination */
  next_cursor?: string;
}

/**
 * Content that can be different types
 */
export interface Content {
  /** Type of content */
  type: "text" | "image" | "file";
  /** Text content if type is text */
  text?: string;
  /** URL if type is image or file */
  url?: string;
  /** MIME type for the content */
  mime_type?: string;
}

/**
 * Request to create a new user
 */
export interface CreateUserRequest {
  /** User's name */
  name: string;
  /** User's email */
  email: string;
  /** Optional user role, defaults to "user" */
  role?: UserRole;
  /** Optional profile picture as content */
  profile_picture?: Content;
} 