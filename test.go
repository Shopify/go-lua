package lua

import (
	"bufio"
	"os"
	"testing"
)

func commonTest(t *testing.T, fileName string, f func(*State, *os.File) *luaClosure, libs ...RegistryFunction) *State {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	l := NewState()
	for _, lib := range libs {
		Require(l, lib.Name, lib.Function, true)
		Pop(l, 1)
	}
	closure := f(l, file)
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

func binaryTest(t *testing.T, fileName string, libs ...RegistryFunction) *State {
	return commonTest(t, fileName, func(l *State, file *os.File) *luaClosure {
		closure, err := l.undump(file, "test")
		if err != nil {
			offset, _ := file.Seek(0, 1)
			t.Error("unexpected error", err, "at file offset", offset)
		}
		return closure
	}, libs...)
}

func sourceTest(t *testing.T, fileName string, libs ...RegistryFunction) *State {
	return commonTest(t, fileName, func(l *State, file *os.File) *luaClosure {
		closure := l.parse(bufio.NewReader(file), "@"+fileName)
		if closure == nil {
			t.Error("closure was nil")
		}
		return closure
	}, libs...)
}
