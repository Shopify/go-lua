package lua

import (
	"fmt"
	"testing"
)

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
