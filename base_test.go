package lua

import "testing"

func TestBase(t *testing.T) { Call(sourceTest(t, "base.lua", RegistryFunction{"_G", BaseOpen}), 0, 0) }

func TestHello(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	LoadString(l, `print("Hello World!")`)
	Call(l, 0, 0)
}
