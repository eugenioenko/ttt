package example

import "testing"

func TestGreet(t *testing.T) {
	got := Greet("World")
	if got != "Hello, World!" {
		t.Errorf("Greet(World) = %q, want %q", got, "Hello, World!")
	}
}

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2,3) should be 5")
	}
}

func TestMultiply(t *testing.T) {
	if Multiply(3, 4) != 12 {
		t.Error("Multiply(3,4) should be 12")
	}
}

func TestDivide(t *testing.T) {
	if Divide(10, 2) != 5 {
		t.Error("Divide(10,2) should be 5")
	}
}

func TestFibonacci(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 0}, {1, 1}, {5, 5}, {10, 55},
	}
	for _, tc := range cases {
		if got := Fibonacci(tc.n); got != tc.want {
			t.Errorf("Fibonacci(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

func TestIsPrime(t *testing.T) {
	if !IsPrime(7) {
		t.Error("7 should be prime")
	}
	if IsPrime(4) {
		t.Error("4 should not be prime")
	}
}
