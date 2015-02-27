package lua_test

import (
	"fmt"
	"github.com/Shopify/go-lua"
)

type step struct {
	name     string
	function interface{}
}

func Example() {
	steps := []step{}
	l := lua.NewState()
	lua.BaseOpen(l)
	_ = lua.NewMetaTable(l, "stepMetaTable")
	lua.SetFunctions(l, []lua.RegistryFunction{{"__newindex", func(l *lua.State) int {
		k, v := lua.CheckString(l, 2), l.ToValue(3)
		steps = append(steps, step{name: k, function: v})
		return 0
	}}}, 0)
	l.PushUserData(steps)
	l.PushValue(-1)
	l.SetGlobal("step")
	lua.SetMetaTableNamed(l, "stepMetaTable")
	lua.LoadString(l, `step.request_tracking_js = function ()
    get(config.domain..'/javascripts/shopify_stats.js')
  end`)
	l.Call(0, 0)
	fmt.Println(steps[0].name)
	// Output: request_tracking_js
}
