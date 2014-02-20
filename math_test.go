package lua

import "testing"

func TestMath(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadFile(l, "fixtures/math.lua", "text")
	Call(l, 0, 0)
}
