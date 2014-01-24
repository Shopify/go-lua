package lua

import "testing"

func TestMath(t *testing.T) {
	Call(binaryTest(t, "math.bin", RegistryFunction{"_G", BaseOpen}, RegistryFunction{"math", MathOpen}), 0, 0)
}
