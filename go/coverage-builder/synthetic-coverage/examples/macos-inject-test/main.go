package main

import (
	"fmt"
	"strings"
)

// Calculator provides basic math operations
type Calculator struct {
	precision int
}

// NewCalculator creates a new calculator
func NewCalculator(precision int) *Calculator {
	return &Calculator{precision: precision}
}

// Add adds two numbers
func (c *Calculator) Add(a, b float64) float64 {
	return a + b
}

// Subtract subtracts b from a
func (c *Calculator) Subtract(a, b float64) float64 {
	return a - b
}

// Multiply multiplies two numbers
func (c *Calculator) Multiply(a, b float64) float64 {
	return a * b
}

// Divide divides a by b
func (c *Calculator) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}

// FormatResult formats a result with precision
func (c *Calculator) FormatResult(value float64) string {
	format := fmt.Sprintf("%%.%df", c.precision)
	return fmt.Sprintf(format, value)
}

// UnusedFunction is never called
func (c *Calculator) UnusedFunction() string {
	return "This function is never tested"
}

// ProcessData processes some data (for synthetic coverage demo)
func ProcessData(data []string) string {
	if len(data) == 0 {
		return "empty"
	}
	return strings.Join(data, ", ")
}

func main() {
	calc := NewCalculator(2)
	result := calc.Add(10, 20)
	fmt.Printf("10 + 20 = %s\n", calc.FormatResult(result))
}