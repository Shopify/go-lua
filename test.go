package lua

import (
	"os"
	"testing"
)

// This is a temporary helper API until the Lua compiler is complete
func BinaryTest(t *testing.T, fileName string, libs ...RegistryFunction) State {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	l := NewState().(*state)
	for _, lib := range libs {
		Require(l, lib.Name, lib.Function, true)
		l.Pop(1)
	}
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
	return l
}
