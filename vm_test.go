package lua

import "testing"

func TestVm(t *testing.T) { Call(sourceTest(t, "fib.lua", RegistryFunction{"_G", BaseOpen}), 0, 0) }
