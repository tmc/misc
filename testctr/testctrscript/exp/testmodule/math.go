package testmodule

// Add returns the sum of two integers
func Add(a, b int) int {
	return a + b
}

// Multiply returns the product of two integers
func Multiply(a, b int) int {
	result := 0
	for i := 0; i < b; i++ {
		result = Add(result, a)
	}
	return result
}