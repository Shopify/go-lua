package lua

import (
	"fmt"
	"testing"
)

func TestVm(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	LoadFile(l, "fixtures/fib.lua", "t")
	Call(l, 0, 0)
}

func TestConcat(t *testing.T) {
	l := NewState()
	BaseOpen(l)
	LoadString(l, "print('hello'..'world')")
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

func TestBit32(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadFile(l, "fixtures/bitwise.lua", "text")
	Call(l, 0, 0)
}

func TestMath(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadFile(l, "fixtures/math.lua", "text")
	Call(l, 0, 0)
}

func TestTableUnpack(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadString(l, "local x, y = table.unpack({-10,0}); assert(x == -10 and y == 0)")
	Call(l, 0, 0)
}

func TestBase(t *testing.T) {
	s := `
    print("hello world\n")
    assert(true)`
	l := NewState()
	BaseOpen(l)
	LoadString(l, s)
	Call(l, 0, 0)
}

func TestGoto(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	LoadFile(l, "fixtures/goto.lua", "text")
	Call(l, 0, 0)
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
