package lua

import (
	"fmt"
	"testing"
)

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

func TestLua(t *testing.T) {
	tests := []string{
		"fib",
		"bitwise",
		"math",
		"goto",
		"closure",
		// "attrib",
	}
	for _, v := range tests {
		t.Log(v)
		l := NewState()
		OpenLibraries(l)
		PushBoolean(l, true)
		SetGlobal(l, "_port")
		// SetHooker(l, func(state *State, ar *Debug) {
		// 	ci := state.callInfo.(*luaCallInfo)
		// 	println(stack(state.stack[ci.base():state.top]))
		// 	println(ci.code[ci.savedPC].String())
		// }, MaskCount, 1)
		LoadFile(l, "fixtures/"+v+".lua", "text")
		if err := ProtectedCall(l, 0, 0, 0); err != nil {
			t.Errorf("'%s' failed: %s", v, err.Error())
		}
	}
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
