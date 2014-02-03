package lua

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

func loadBinary(l *State, t *testing.T, fileName string) *luaClosure {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	closure, err := l.undump(file, "test")
	if err != nil {
		offset, _ := file.Seek(0, 1)
		t.Error("unexpected error", err, "at file offset", offset)
	}
	for i := range closure.upValues {
		closure.upValues[i] = l.newUpValue()
	}
	if len(closure.upValues) == 1 {
		globals := l.global.registry.atInt(RegistryIndexGlobals)
		closure.upValues[0].setValue(globals)
	}
	return closure
}

func loadSource(l *State, t *testing.T, fileName string) *luaClosure {
	file, err := os.Open(fileName)
	if err != nil {
		t.Fatal("couldn't open " + fileName)
	}
	closure := l.parse(bufio.NewReader(file), "@"+fileName)
	if closure == nil {
		t.Error("closure was nil")
	}
	for i := range closure.upValues {
		closure.upValues[i] = l.newUpValue()
	}
	if len(closure.upValues) == 1 {
		globals := l.global.registry.atInt(RegistryIndexGlobals)
		closure.upValues[0].setValue(globals)
	}
	return closure
}

func TestParser(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	// SetHook(l, func(state *State, ar *Debug) {
	// 	printStack(state.stack[state.callInfo.(*luaCallInfo).base():state.top])
	// 	println(state.callInfo.(*luaCallInfo).code[state.callInfo.(*luaCallInfo).savedPC].String())
	// }, MaskCount, 1)
	bin := loadBinary(l, t, "fib.bin")
	Pop(l, 1)
	closure := loadSource(l, t, "fib.lua")
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
	compareClosures(t, bin, closure)
	Call(l, 0, 0)
}

func TestParserExhaustively(t *testing.T) {
	t.Skip()
	l := NewState()
	matches, err := filepath.Glob("/Users/fbogsany/Projects/Lua/lua-5.2.2-tests/*.lua")
	if err != nil {
		t.Fatal(err)
	}
	blackList := map[string]bool{"all.lua": true, "main.lua": true, "bitwise.lua": true}
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
	t.Log("Parsing " + source)
	bin := loadBinary(l, t, strings.TrimSuffix(source, ".lua")+".bin")
	Pop(l, 1)
	src := loadSource(l, t, source)
	Pop(l, 1)
	t.Log(source)
	compareClosures(t, src, bin)
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
	if reflect.TypeOf(x).Kind() == reflect.Slice && reflect.ValueOf(y).Len() == 0 && reflect.ValueOf(x).Len() == 0 {
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
	// expectEqual(t, a.source, b.source, "source")
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
