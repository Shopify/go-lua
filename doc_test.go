package lua_test

import (
	"github.com/Shopify/go-lua"
)

// This example receives a variable number of numerical arguments and returns their average and sum.
func ExampleFunction(l *lua.State) int {
	n := l.Top() // Number of arguments.
	var sum float64
	for i := 1; i <= n; i++ {
		f, ok := l.ToNumber(i)
		if !ok {
			l.PushString("incorrect argument")
			l.Error()
		}
		sum += f
	}
	l.PushNumber(sum / float64(n)) // First result.
	l.PushNumber(sum)              // Second result.
	return 2                       // Result count.
}
