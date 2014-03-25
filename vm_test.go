package lua

import (
	"fmt"
	"testing"
)

func testString(t *testing.T, s string)  { testStringHelper(t, s, false) }
func traceString(t *testing.T, s string) { testStringHelper(t, s, true) }

func testStringHelper(t *testing.T, s string, trace bool) {
	l := NewState()
	OpenLibraries(l)
	LoadString(l, s)
	if trace {
		SetHooker(l, func(state *State, ar *Debug) {
			ci := state.callInfo.(*luaCallInfo)
			p := state.stack[ci.function()].(*luaClosure).prototype
			println(stack(state.stack[ci.base():state.top]))
			println(ci.code[ci.savedPC].String(), p.source, p.lineInfo[ci.savedPC])
		}, MaskCount, 1)
	}
	Call(l, 0, 0)
}

func TestProtectedCall(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	SetHooker(l, func(state *State, ar *Debug) {
		ci := state.callInfo.(*luaCallInfo)
		_ = stack(state.stack[ci.base():state.top])
		_ = ci.code[ci.savedPC].String()
	}, MaskCount, 1)
	LoadString(l, "assert(not pcall(bit32.band, {}))")
	Call(l, 0, 0)
}

func TestLua(t *testing.T) {
	tests := []string{
		"attrib",
		"bitwise",
		"closure",
		"events",
		"fib",
		"goto",
		"locals",
		"math",
		//"sort",
		"strings",
		//"vararg",
	}
	for _, v := range tests {
		t.Log(v)
		l := NewState()
		OpenLibraries(l)
		for _, s := range []string{"_port", "_no32", "_noformatA"} {
			PushBoolean(l, true)
			SetGlobal(l, s)
		}
		//		SetHooker(l, func(state *State, ar *Debug) {
		//			ci := state.callInfo.(*luaCallInfo)
		//			p := state.stack[ci.function()].(*luaClosure).prototype
		//			println(stack(state.stack[ci.base():state.top]))
		//			println(ci.code[ci.savedPC].String(), p.source, p.lineInfo[ci.savedPC])
		//		}, MaskCount, 1)
		LoadFile(l, "fixtures/"+v+".lua", "text")
		if err := ProtectedCall(l, 0, 0, 0); err != nil {
			t.Errorf("'%s' failed: %s", v, err.Error())
		}
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
	PushString(l, "hello")
	Remove(l, -1)
	PushNil(l)
	Remove(l, -1)
}

func TestTableNext(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	CreateTable(l, 10, 0)
	for i := 1; i <= 4; i++ {
		PushInteger(l, i)
		PushValue(l, -1)
		SetTable(l, -3)
	}
	if length := LengthEx(l, -1); length != 4 {
		t.Errorf("expected table length to be 4, but was %d", length)
	}
	count := 0
	for PushNil(l); Next(l, -2); count++ {
		if k, v := CheckInteger(l, -2), CheckInteger(l, -1); k != v {
			t.Errorf("key %d != value %d", k, v)
		}
		Pop(l, 1)
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
	PushGoFunction(l, func(l *State) int {
		if Top(l) == 0 {
			t.Error("error handler received no arguments")
		} else if errorMessage, ok := ToString(l, -1); !ok {
			t.Errorf("error handler received %s instead of string", TypeNameOf(l, -1))
		} else if errorMessage != program+":1: error" {
			t.Errorf("error handler received '%s' instead of 'error'", errorMessage)
		}
		errorHandled = true
		return 1
	})
	LoadString(l, program)
	ProtectedCall(l, 0, 0, -2)
	if !errorHandled {
		t.Error("error not handled")
	}
}

func Example() {
	type step struct {
		name     string
		function interface{}
	}
	steps := []step{}
	l := NewState()
	BaseOpen(l)
	_ = NewMetaTable(l, "stepMetaTable")
	SetFunctions(l, []RegistryFunction{{"__newindex", func(l *State) int {
		k, v := CheckString(l, 2), ToValue(l, 3)
		steps = append(steps, step{name: k, function: v})
		return 0
	}}}, 0)
	PushUserData(l, steps)
	PushValue(l, -1)
	SetGlobal(l, "step")
	SetMetaTableNamed(l, "stepMetaTable")
	LoadString(l, `step.request_tracking_js = function ()
	  get(config.domain..'/javascripts/shopify_stats.js')
	end`)
	Call(l, 0, 0)
	fmt.Println(steps[0].name)
	// Output: request_tracking_js
}
