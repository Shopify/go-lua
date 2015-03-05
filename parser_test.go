package lua

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

func load(l *State, t *testing.T, fileName string) *luaClosure {
	if err := LoadFile(l, fileName, "bt"); err != nil {
		return nil
	}
	return l.ToValue(-1).(*luaClosure)
}

func TestParser(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	bin := load(l, t, "fixtures/fib.bin")
	l.Pop(1)
	closure := load(l, t, "fixtures/fib.lua")
	p := closure.prototype
	if p == nil {
		t.Fatal("prototype was nil")
	}
	validate("@fixtures/fib.lua", p.source, "as source file name", t)
	if !p.isVarArg {
		t.Error("expected main function to be var arg, but wasn't")
	}
	if len(closure.upValues) != len(closure.prototype.upValues) {
		t.Error("upvalue count doesn't match", len(closure.upValues), "!=", len(closure.prototype.upValues))
	}
	compareClosures(t, bin, closure)
	l.Call(0, 0)
}

func TestEmptyString(t *testing.T) {
	l := NewState()
	if err := LoadString(l, ""); err != nil {
		t.Fatal(err.Error())
	}
	l.Call(0, 0)
}

func TestParserExhaustively(t *testing.T) {
	_, err := exec.LookPath("luac")
	if err != nil {
		t.Skipf("exhaustively testing the parser requires luac: %s", err)
	}
	l := NewState()
	matches, err := filepath.Glob(filepath.Join("lua-tests", "*.lua"))
	if err != nil {
		t.Fatal(err)
	}
	blackList := map[string]bool{"math.lua": true}
	for _, source := range matches {
		if _, ok := blackList[filepath.Base(source)]; ok {
			continue
		}
		protectedTestParser(l, t, source)
	}
}

func protectedTestParser(l *State, t *testing.T, source string) {
	defer func() {
		if x := recover(); x != nil {
			t.Error(x)
			t.Log(string(debug.Stack()))
		}
	}()
	t.Log("Compiling " + source)
	binary := strings.TrimSuffix(source, ".lua") + ".bin"
	if err := exec.Command("luac", "-o", binary, source).Run(); err != nil {
		t.Fatalf("luac failed to compile %s: %s", source, err)
	}
	t.Log("Parsing " + source)
	bin := load(l, t, binary)
	l.Pop(1)
	src := load(l, t, source)
	l.Pop(1)
	t.Log(source)
	compareClosures(t, src, bin)
}

func expectEqual(t *testing.T, x, y interface{}, m string) {
	if x != y {
		t.Errorf("%s doesn't match: %v, %v\n", m, x, y)
	}
}

func expectDeepEqual(t *testing.T, x, y interface{}, m string) bool {
	if reflect.DeepEqual(x, y) {
		return true
	}
	if reflect.TypeOf(x).Kind() == reflect.Slice && reflect.ValueOf(y).Len() == 0 && reflect.ValueOf(x).Len() == 0 {
		return true
	}
	t.Errorf("%s doesn't match: %v, %v\n", m, x, y)
	return false
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
	if !expectDeepEqual(t, a.code, b.code, "code") {
		for i := range a.code {
			if a.code[i] != b.code[i] {
				t.Errorf("%d: %v != %v\n", a.lineInfo[i], a.code[i], b.code[i])
			}
		}
		for _, i := range []int{3, 197, 198, 199, 200, 201} {
			t.Errorf("%d: %#v, %#v\n", i, a.constants[i], b.constants[i])
		}
		for _, i := range []int{202, 203, 204} {
			t.Errorf("%d: %#v\n", i, b.constants[i])
		}
	}
	if !expectDeepEqual(t, a.constants, b.constants, "constants") {
		for i := range a.constants {
			if a.constants[i] != b.constants[i] {
				t.Errorf("%d: %#v != %#v\n", i, a.constants[i], b.constants[i])
			}
		}
	}
	expectDeepEqual(t, a.lineInfo, b.lineInfo, "line info")
	expectDeepEqual(t, a.upValues, b.upValues, "upvalues")
	expectDeepEqual(t, a.localVariables, b.localVariables, "local variables")
	expectEqual(t, len(a.prototypes), len(b.prototypes), "prototypes length")
	for i := range a.prototypes {
		comparePrototypes(t, &a.prototypes[i], &b.prototypes[i])
	}
}
