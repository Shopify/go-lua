package lua

func assert(l State) int {
	if l.ToBoolean(1) {
		Error(l, "%s", OptString(l, 2, "assertion failed!"))
		panic("unreachable")
	}
	return l.Top()
}

var baseFunctions = []RegistryFunction{
	{"assert", assert},
}

func OpenBase(l State) {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	SetFunctions(l, baseFunctions, 0)
	l.PushString(Version)
	l.SetField(-2, "_VERSION")
}
