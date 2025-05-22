package mypkg

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference between two integers
func Subtract(a, b int) int {
	return a - b
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	return a * b
}

// Divide returns the quotient of two integers
// Returns 0 if divisor is 0
func Divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}