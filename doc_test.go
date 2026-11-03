package lua_test

import (
	"fmt"

	"github.com/Shopify/go-lua"
)

// This example receives a variable number of numerical arguments and returns their average and sum.
func ExampleFunction() {
	averageAndSum := func(l *lua.State) int {
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

	l := lua.NewState()
	l.PushGoFunction(averageAndSum)
	l.PushNumber(1)
	l.PushNumber(2)
	l.PushNumber(3)
	l.Call(3, 2)
	average, _ := l.ToNumber(-2)
	sum, _ := l.ToNumber(-1)
	fmt.Println(average, sum)
	// Output: 2 6
}
