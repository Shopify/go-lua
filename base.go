package lua

import "os"

func _print(l State) int {
	n := l.Top()
	l.Global("tostring")
	for i := 1; i <= n; i++ {
		l.PushValue(-1) // function to be called
		l.PushValue(i)  // value to print
		l.Call(1, 1)
		s, ok := l.ToString(-1)
		if !ok {
			Error(l, "'tostring' must return a string to 'print'")
			panic("unreachable")
		}
		if i > 1 {
			os.Stdout.WriteString("\t")
		}
		os.Stdout.WriteString(s)
		l.Pop(1) // pop result
	}
	os.Stdout.WriteString("\n")
	os.Stdout.Sync()
	return 0
}

func _assert(l State) int {
	if !l.ToBoolean(1) {
		Error(l, "%s", OptString(l, 2, "assertion failed!"))
		panic("unreachable")
	}
	return l.Top()
}

func _toString(l State) int {
	CheckAny(l, 1)
	ToString(l, 1)
	return 1
}

var baseFunctions = []RegistryFunction{
	{"assert", _assert},
	{"print", _print},
	{"tostring", _toString},
}

func OpenBase(l State) {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	SetFunctions(l, baseFunctions, 0)
	l.PushString(Version)
	l.SetField(-2, "_VERSION")
}
