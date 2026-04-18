package main

import (
	"errors"
	"log"
)

// ErrDivisionByZero is returned when dividing by zero.
var ErrDivisionByZero = errors.New("division by zero")

func main() {
	log.Println("Hello from go-template!")
}

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference of two integers.
func Subtract(a, b int) int {
	return a - b
}

// Multiply returns the product of two integers.
func Multiply(a, b int) int {
	return a * b
}

// Divide returns the quotient of two integers.
// Returns 0 and an error if the divisor is zero.
func Divide(a, b int) (int, error) {
	if b == 0 {
		return 0, ErrDivisionByZero
	}

	return a / b, nil
}

// Abs returns the absolute value of an integer.
func Abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}

// Clamp restricts a value to a given range [lo, hi].
func Clamp(value, lo, hi int) int {
	if value < lo {
		return lo
	}

	if value > hi {
		return hi
	}

	return value
}
