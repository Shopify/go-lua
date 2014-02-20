package lua

import "testing"

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
