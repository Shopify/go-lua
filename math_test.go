package lua

import "testing"

func TestMath(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadString(l, "print(math.abs(-math.pi))")
	Call(l, 0, 0)
}
