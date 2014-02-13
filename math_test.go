package lua

import "testing"

func TestMath(t *testing.T) {
	t.Skip()
	l := NewState()
	OpenLibraries(l)
	// SetHooker(l, func(state *State, ar *Debug) {
	// 	ci := state.callInfo.(*luaCallInfo)
	// 	printStack(state.stack[ci.base():state.top])
	// 	println(ci.code[ci.savedPC].String())
	// 	p := state.stack[ci.function()].(*luaClosure).prototype
	// 	println(p.source, p.lineInfo[ci.savedPC])
	// }, MaskCount, 1)
	// LoadString(l, "print(math.abs(-math.pi))")
	LoadFile(l, "fixtures/math.lua", "text")
	Call(l, 0, 0)
}
