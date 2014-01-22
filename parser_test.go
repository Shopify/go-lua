package lua

import (
	"bufio"
	"os"
	"reflect"
	"testing"
)

func loadFibBinary(l *State, t *testing.T) *luaClosure {
	fileName := "fib.bin"
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	closure, err := l.undump(file, "test")
	if err != nil {
		offset, _ := file.Seek(0, 1)
		t.Error("unexpected error", err, "at file offset", offset)
	}
	return closure
}

func loadFibSource(l *State, t *testing.T) *luaClosure {
	fileName := "fib.lua"
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	closure := l.parse(bufio.NewReader(file), "@"+fileName)
	if closure == nil {
		t.Error("closure was nil")
	}
	return closure
}

func TestParser(t *testing.T) {
	l := NewState()
	bin := loadFibBinary(l, t)
	Pop(l, 1)
	closure := loadFibSource(l, t)
	p := closure.prototype
	if p == nil {
		t.Fatal("prototype was nil")
	}
	validate("@fib.lua", p.source, "as source file name", t)
	if !p.isVarArg {
		t.Error("expected main function to be var arg, but wasn't")
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
	compareClosures(t, bin, closure)
	Call(l, 0, 0)
}

func expectEqual(t *testing.T, x, y interface{}, m string) {
	if x != y {
		t.Errorf("%s doesn't match: %v, %v\n", m, x, y)
	}
}

func expectDeepEqual(t *testing.T, x, y interface{}, m string) {
	if reflect.DeepEqual(x, y) {
		return
	}
	if reflect.TypeOf(x).Kind() == reflect.Slice && reflect.ValueOf(x).Len() == reflect.ValueOf(y).Len() {
		return
	}
	t.Errorf("%s doesn't match: %v, %v\n", m, x, y)
}

func compareClosures(t *testing.T, a, b *luaClosure) {
	expectEqual(t, a.upValueCount(), b.upValueCount(), "upvalue count")
	comparePrototypes(t, a.prototype, b.prototype)
}

func comparePrototypes(t *testing.T, a, b *prototype) {
	expectEqual(t, a.isVarArg, b.isVarArg, "var arg")
	expectEqual(t, a.lineDefined, b.lineDefined, "line defined")
	expectEqual(t, a.lastLineDefined, b.lastLineDefined, "last line defined")
	expectEqual(t, a.parameterCount, b.parameterCount, "parameter count")
	expectEqual(t, a.maxStackSize, b.maxStackSize, "max stack size")
	expectEqual(t, a.source, b.source, "source")
	expectEqual(t, len(a.code), len(b.code), "code length")
	expectDeepEqual(t, a.code, b.code, "code")
	expectDeepEqual(t, a.constants, b.constants, "constants")
	expectDeepEqual(t, a.lineInfo, b.lineInfo, "line info")
	expectDeepEqual(t, a.upValues, b.upValues, "upvalues")
	expectDeepEqual(t, a.localVariables, b.localVariables, "local variables")
	expectEqual(t, len(a.prototypes), len(b.prototypes), "prototypes length")
	for i := range a.prototypes {
		comparePrototypes(t, &a.prototypes[i], &b.prototypes[i])
	}
}
