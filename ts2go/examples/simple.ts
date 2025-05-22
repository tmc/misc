/**
 * A basic type alias for strings
 */
export type ID = string;

/**
 * Represents a point in 2D space
 */
export interface Point {
  x: number;
  y: number;
}

/**
 * Represents a rectangle defined by two points
 */
export interface Rectangle {
  /** Top-left point of the rectangle */
  topLeft: Point;
  /** Bottom-right point of the rectangle */
  bottomRight: Point;
}

/**
 * Represents a user in the system
 */
export interface User {
  /** Unique identifier for the user */
  id: ID;
  /** User's full name */
  name: string;
  /** User's email address (optional) */
  email?: string;
  /** User's age */
  age: number;
  /** Array of role names */
  roles: string[];
}

/**
 * Status values for an order
 */
export type OrderStatus = "pending" | "processing" | "shipped" | "delivered" | "cancelled";

/**
 * Version of the API
 */
export const API_VERSION = "1.0.0";

/**
 * Maximum number of API requests per minute
 */
export const MAX_REQUESTS_PER_MINUTE = 100;

/**
 * Whether debug mode is enabled
 */
export const DEBUG_MODE = false; 