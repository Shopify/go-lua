package lua

import "testing"

func TestBase(t *testing.T) {
	Call(BinaryTest(t, "base.bin", RegistryFunction{"_G", BaseOpen}), 0, 0)
}
