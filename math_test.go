package lua

import (
	"os"
	"testing"
)

func TestMath(t *testing.T) {
	file, err := os.Open("math.bin")
	if err != nil {
		t.Fatal("couldn't open math.bin")
	}
	l := NewState().(*state)
	OpenBase(l)
	Require(l, "math", OpenMath, true)
	l.Pop(1)
	closure, err := l.undump(file, "test")
	if err != nil {
		offset, _ := file.Seek(0, 1)
		t.Error("unexpected error", err, "at file offset", offset)
	}
	if len(closure.upValues) != len(closure.prototype.upValues) {
		t.Error("upvalue count doesn't match", len(closure.upValues), "!=", len(closure.prototype.upValues))
	}
	for i := range closure.upValues {
		closure.upValues[i] = l.newUpValue()
	}
	if len(closure.upValues) == 1 {
		globals := l.global.registry.atInt(RegistryIndexGlobals)
		closure.upValues[0].setValue(globals)
	}
	l.Call(0, 0)
}
