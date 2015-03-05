package lua

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testString(t *testing.T, s string)  { testStringHelper(t, s, false) }
func traceString(t *testing.T, s string) { testStringHelper(t, s, true) }

func testStringHelper(t *testing.T, s string, trace bool) {
	l := NewState()
	OpenLibraries(l)
	LoadString(l, s)
	if trace {
		SetDebugHook(l, func(state *State, ar Debug) {
			ci := state.callInfo
			p := state.prototype(ci)
			println(stack(state.stack[ci.base():state.top]))
			println(ci.code[ci.savedPC].String(), p.source, p.lineInfo[ci.savedPC])
		}, MaskCount, 1)
	}
	l.Call(0, 0)
}

func TestProtectedCall(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	SetDebugHook(l, func(state *State, ar Debug) {
		ci := state.callInfo
		_ = stack(state.stack[ci.base():state.top])
		_ = ci.code[ci.savedPC].String()
	}, MaskCount, 1)
	LoadString(l, "assert(not pcall(bit32.band, {}))")
	l.Call(0, 0)
}

func TestLua(t *testing.T) {
	tests := []struct {
		name    string
		nonPort bool
	}{
		{name: "attrib", nonPort: true},
		// {name: "big"},
		{name: "bitwise"},
		// {name: "calls"},
		// {name: "checktable"},
		{name: "closure"},
		// {name: "code"},
		// {name: "constructs"},
		// {name: "db"},
		// {name: "errors"},
		{name: "events"},
		// {name: "files"},
		// {name: "gc"},
		{name: "goto"},
		// {name: "literals"},
		{name: "locals"},
		// {name: "main"},
		{name: "math"},
		// {name: "nextvar"},
		// {name: "pm"},
		{name: "sort", nonPort: true}, // sort.lua depends on os.clock(), which is not yet implemented on Windows.
		{name: "strings"},
		// {name: "vararg"},
		// {name: "verybig"},
	}
	for _, v := range tests {
		if v.nonPort && runtime.GOOS == "windows" {
			t.Skipf("'%s' skipped because it's non-portable & we're running Windows", v.name)
		}
		t.Log(v)
		l := NewState()
		OpenLibraries(l)
		for _, s := range []string{"_port", "_no32", "_noformatA"} {
			l.PushBoolean(true)
			l.SetGlobal(s)
		}
		if v.nonPort {
			l.PushBoolean(false)
			l.SetGlobal("_port")
		}
		// l.SetDebugHook(func(state *State, ar Debug) {
		// 	ci := state.callInfo.(*luaCallInfo)
		// 	p := state.prototype(ci)
		// 	println(stack(state.stack[ci.base():state.top]))
		// 	println(ci.code[ci.savedPC].String(), p.source, p.lineInfo[ci.savedPC])
		// }, MaskCount, 1)
		l.Global("debug")
		l.Field(-1, "traceback")
		traceback := l.Top()
		// t.Logf("%#v", l.ToValue(traceback))
		if err := LoadFile(l, filepath.Join("lua-tests", v.name+".lua"), "text"); err != nil {
			t.Errorf("'%s' failed: %s", v.name, err.Error())
		}
		// l.Call(0, 0)
		if err := l.ProtectedCall(0, 0, traceback); err != nil {
			t.Errorf("'%s' failed: %s", v.name, err.Error())
		}
	}
}

func benchmarkSort(b *testing.B, program string) {
	l := NewState()
	OpenLibraries(l)
	s := `a = {}
		for i=1,%d do
			a[i] = math.random()
		end`
	LoadString(l, fmt.Sprintf(s, b.N))
	if err := l.ProtectedCall(0, 0, 0); err != nil {
		b.Error(err.Error())
	}
	LoadString(l, program)
	b.ResetTimer()
	if err := l.ProtectedCall(0, 0, 0); err != nil {
		b.Error(err.Error())
	}
}

func BenchmarkSort(b *testing.B) { benchmarkSort(b, "table.sort(a)") }
func BenchmarkSort2(b *testing.B) {
	benchmarkSort(b, "i = 0; table.sort(a, function(x,y) i=i+1; return y<x end)")
}

func BenchmarkFibonnaci(b *testing.B) {
	l := NewState()
	s := `return function(n)
			if n == 0 then
				return 0
			elseif n == 1 then
				return 1
			end
			local n0, n1 = 0, 1
			for i = n, 2, -1 do
				local tmp = n0 + n1
				n0 = n1
				n1 = tmp
			end
			return n1
		end`
	LoadString(l, s)
	if err := l.ProtectedCall(0, 1, 0); err != nil {
		b.Error(err.Error())
	}
	l.PushInteger(b.N)
	b.ResetTimer()
	if err := l.ProtectedCall(1, 1, 0); err != nil {
		b.Error(err.Error())
	}
}

func TestVarArgMeta(t *testing.T) {
	s := `function f(t, ...) return t, {...} end
		local a = setmetatable({}, {__call = f})
		local x, y = a(table.unpack{"a", 1})
		assert(#x == 0)
		assert(#y == 2 and y[1] == "a" and y[2] == 1)`
	testString(t, s)
}

func TestCanRemoveNilObjectFromStack(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("failed to remove `nil`, %v", r)
		}
	}()

	l := NewState()
	l.PushString("hello")
	l.Remove(-1)
	l.PushNil()
	l.Remove(-1)
}

func TestTableNext(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	l.CreateTable(10, 0)
	for i := 1; i <= 4; i++ {
		l.PushInteger(i)
		l.PushValue(-1)
		l.SetTable(-3)
	}
	if length := LengthEx(l, -1); length != 4 {
		t.Errorf("expected table length to be 4, but was %d", length)
	}
	count := 0
	for l.PushNil(); l.Next(-2); count++ {
		if k, v := CheckInteger(l, -2), CheckInteger(l, -1); k != v {
			t.Errorf("key %d != value %d", k, v)
		}
		l.Pop(1)
	}
	if count != 4 {
		t.Errorf("incorrect iteration count %d in Next()", count)
	}
}

func TestError(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	errorHandled := false
	program := "error('error')"
	l.PushGoFunction(func(l *State) int {
		if l.Top() == 0 {
			t.Error("error handler received no arguments")
		} else if errorMessage, ok := l.ToString(-1); !ok {
			t.Errorf("error handler received %s instead of string", TypeNameOf(l, -1))
		} else if errorMessage != chunkID(program)+":1: error" {
			t.Errorf("error handler received '%s' instead of 'error'", errorMessage)
		}
		errorHandled = true
		return 1
	})
	LoadString(l, program)
	l.ProtectedCall(0, 0, -2)
	if !errorHandled {
		t.Error("error not handled")
	}
}

func TestErrorf(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	program := "-- script that is bigger than the max ID size\nhelper()\n" + strings.Repeat("--", idSize)
	expectedErrorMessage := chunkID(program) + ":2: error"
	l.PushGoFunction(func(l *State) int {
		Errorf(l, "error")
		return 0
	})
	l.SetGlobal("helper")
	errorHandled := false
	l.PushGoFunction(func(l *State) int {
		if l.Top() == 0 {
			t.Error("error handler received no arguments")
		} else if errorMessage, ok := l.ToString(-1); !ok {
			t.Errorf("error handler received %s instead of string", TypeNameOf(l, -1))
		} else if errorMessage != expectedErrorMessage {
			t.Errorf("error handler received '%s' instead of '%s'", errorMessage, expectedErrorMessage)
		}
		errorHandled = true
		return 1
	})
	LoadString(l, program)
	l.ProtectedCall(0, 0, -2)
	if !errorHandled {
		t.Error("error not handled")
	}
}
