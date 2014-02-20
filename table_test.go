package lua

import "testing"

func TestTableUnpack(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadString(l, "local x, y = table.unpack({-10,0}); assert(x == -10 and y == 0)")
	Call(l, 0, 0)
}
