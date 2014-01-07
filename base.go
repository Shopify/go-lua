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

func _error(l State) int {
	level := OptInteger(l, 2, 1)
	l.SetTop(1)
	if l.IsString(1) && level > 0 {
		Where(l, level)
		l.PushValue(1)
		l.Concat(2)
	}
	l.Error()
	panic("unreachable")
}

func _getMetaTable(l State) int {
	CheckAny(l, 1)
	if !l.MetaTable(1) {
		l.PushNil()
		return 1
	}
	MetaField(l, 1, "__metatable")
	return 1
}

func _setMetaTable(l State) int {
	t := l.Type(2)
	CheckType(l, 1, TypeTable)
	ArgumentCheck(l, t == TypeNil || t == TypeTable, 2, "nil or table expected")
	if MetaField(l, 1, "__metatable") {
		Error(l, "cannot change a protected metatable")
	}
	l.SetTop(2)
	l.SetMetaTable(1)
	return 1
}

func _rawEqual(l State) int {
	CheckAny(l, 1)
	CheckAny(l, 2)
	l.PushBoolean(l.RawEqual(1, 2))
	return 1
}

func _rawLength(l State) int {
	t := l.Type(1)
	ArgumentCheck(l, t == TypeTable || t == TypeString, 1, "table or string expected")
	l.PushInteger(l.RawLength(1))
	return 1
}

func _rawGet(l State) int {
	CheckType(l, 1, TypeTable)
	CheckAny(l, 2)
	l.SetTop(2)
	l.RawGet(1)
	return 1
}

func _rawSet(l State) int {
	CheckType(l, 1, TypeTable)
	CheckAny(l, 2)
	CheckAny(l, 3)
	l.SetTop(3)
	l.RawSet(1)
	return 1
}

func _type(l State) int {
	CheckAny(l, 1)
	l.PushString(l.TypeName(1))
	return 1
}

func _next(l State) int {
	CheckType(l, 1, TypeTable)
	l.SetTop(2)
	if l.Next(1) {
		return 2
	}
	l.PushNil()
	return 1
}

func _assert(l State) int {
	if !l.ToBoolean(1) {
		Error(l, "%s", OptString(l, 2, "assertion failed!"))
		panic("unreachable")
	}
	return l.Top()
}

func _select(l State) int {
	n := l.Top()
	if l.Type(1) == TypeString {
		if s, _ := l.ToString(1); s[0] == '#' {
			l.PushInteger(n - 1)
			return 1
		}
	}
	i := CheckInteger(l, 1)
	if i < 0 {
		i = n + i
	} else if i > n {
		i = n
	}
	ArgumentCheck(l, 1 <= i, 1, "index out of range")
	return n - i
}

func _toString(l State) int {
	CheckAny(l, 1)
	ToString(l, 1)
	return 1
}

var baseFunctions = []RegistryFunction{
	{"assert", _assert},

	{"next", _next},
	// {"pairs", _pairs},
	// {"pcall", _pcall},
	{"print", _print},
	{"rawequal", _rawEqual},
	{"rawlen", _rawLength},
	{"rawget", _rawGet},
	{"rawset", _rawSet},
	{"select", _select},
	{"setmetatable", _setMetaTable},
	// {"tonumber", _toNumber},
	{"tostring", _toString},
	{"type", _type},
	// {"xpcall", _xpcall},
}

func OpenBase(l State) {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	SetFunctions(l, baseFunctions, 0)
	l.PushString(Version)
	l.SetField(-2, "_VERSION")
}
