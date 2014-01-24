package lua

import "testing"

func TestVm(t *testing.T) { Call(BinaryTest(t, "fib.bin", RegistryFunction{"_G", BaseOpen}), 0, 0) }
