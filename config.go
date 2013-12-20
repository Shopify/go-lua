package lua

const (
	maxStack     = 1000000
	maxCallCount = 200
	errorStackSize = maxStack + 200
	extraStack     = 5
	basicStackSize = 2 * MinStack
	maxTagLoop = 100
)
