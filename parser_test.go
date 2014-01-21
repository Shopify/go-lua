package lua

import (
	"bufio"
	"os"
	"testing"
)

func TestParser(t *testing.T) {
	filename := "fib.lua"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal("couldn't open " + filename)
	}
	l := NewState()
	closure := l.parse(bufio.NewReader(file), "@"+filename)
	// if err != nil {
	//   offset, _ := file.Seek(0, 1)
	//   t.Error("unexpected error", err, "at file offset", offset)
	// }
	if closure == nil {
		t.Error("closure was nil")
	}
	p := closure.prototype
	if p == nil {
		t.Fatal("prototype was nil")
	}
	validate("@"+filename, p.source, "as source file name", t)
	// validate(23, len(p.code), "instructions", t)
	// validate(8, len(p.constants), "constants", t)
	// validate(4, len(p.prototypes), "prototypes", t)
	// validate(1, len(p.upValues), "upvalues", t)
	// validate(0, len(p.localVariables), "local variables", t)
	// validate(0, p.parameterCount, "parameters", t)
	// validate(4, p.maxStackSize, "stack slots", t)
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
	// TODO call
}
