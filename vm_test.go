package lua

import "testing"

func TestVm(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	LoadFile(l, "fib.lua", "t")
	Call(l, 0, 0)
}
