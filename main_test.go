package main

import (
	"errors"
	"testing"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{name: "positive numbers", a: 1, b: 2, want: 3},
		{name: "negative numbers", a: -1, b: -2, want: -3},
		{name: "zero", a: 0, b: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Add(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSubtract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{name: "positive result", a: 5, b: 3, want: 2},
		{name: "negative result", a: 3, b: 5, want: -2},
		{name: "zero result", a: 4, b: 4, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Subtract(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Subtract(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMultiply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{name: "positive numbers", a: 3, b: 4, want: 12},
		{name: "negative and positive", a: -3, b: 4, want: -12},
		{name: "both negative", a: -3, b: -4, want: 12},
		{name: "multiply by zero", a: 5, b: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Multiply(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Multiply(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDivide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		a       int
		b       int
		want    int
		wantErr error
	}{
		{name: "even division", a: 10, b: 2, want: 5, wantErr: nil},
		{name: "integer truncation", a: 7, b: 2, want: 3, wantErr: nil},
		{name: "negative dividend", a: -10, b: 3, want: -3, wantErr: nil},
		{name: "division by zero", a: 5, b: 0, want: 0, wantErr: ErrDivisionByZero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Divide(tt.a, tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Divide(%d, %d) error = %v, wantErr %v", tt.a, tt.b, err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("Divide(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		want int
	}{
		{name: "positive", n: 5, want: 5},
		{name: "negative", n: -5, want: 5},
		{name: "zero", n: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Abs(tt.n)
			if got != tt.want {
				t.Errorf("Abs(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		lo    int
		hi    int
		want  int
	}{
		{name: "within range", value: 5, lo: 0, hi: 10, want: 5},
		{name: "below min", value: -5, lo: 0, hi: 10, want: 0},
		{name: "above max", value: 15, lo: 0, hi: 10, want: 10},
		{name: "at min boundary", value: 0, lo: 0, hi: 10, want: 0},
		{name: "at max boundary", value: 10, lo: 0, hi: 10, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Clamp(tt.value, tt.lo, tt.hi)
			if got != tt.want {
				t.Errorf("Clamp(%d, %d, %d) = %d, want %d", tt.value, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}
