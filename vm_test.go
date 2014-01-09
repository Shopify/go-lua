package lua

import "testing"

func TestVm(t *testing.T) {
	BinaryTest(t, "fib.bin").Call(0, 0)
}
