package lua

import "testing"

func TestVm(t *testing.T) {
	Call(BinaryTest(t, "fib.bin"), 0, 0)
}
