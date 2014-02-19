package lua

import "testing"

func TestBit32(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadFile(l, "fixtures/bitwise.lua", "text")
	Call(l, 0, 0)
}
