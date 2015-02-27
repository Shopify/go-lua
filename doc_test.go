package lua_test

import (
	"github.com/Shopify/go-lua"
)

// This example receives a variable number of numerical arguments and returns their average and sum.
func ExampleFunction(l *lua.State) int {
	n := lua.Top(l) // Number of arguments.
	var sum float64
	for i := 1; i <= n; i++ {
		f, ok := lua.ToNumber(l, i)
		if !ok {
			lua.PushString(l, "incorrect argument")
			lua.Error(l)
		}
		sum += f
	}
	lua.PushNumber(l, sum/float64(n)) // First result.
	lua.PushNumber(l, sum)            // Second result.
	return 2                          // Result count.
}
