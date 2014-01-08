package lua

import (
	"os"
	"runtime"
	"strconv"
)

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

func _toNumber(l State) int {
	if l.IsNoneOrNil(2) { // standard conversion
		if n, ok := l.ToNumber(1); ok {
			l.PushNumber(n)
			return 1
		}
		CheckAny(l, 1)
	} else {
		s := CheckString(l, 1)
		base := CheckInteger(l, 2)
		ArgumentCheck(l, 2 <= base && base <= 36, 2, "base out of range")
		if i, err := strconv.ParseInt(s, base, 64); err == nil { // TODO strings.TrimSpace(s)?
			l.PushNumber(float64(i))
			return 1
		}
	}
	l.PushNil()
	return 1
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

func _collectGarbage(l State) int {
	switch opt, _ := OptString(l, 1, "collect"), OptInteger(l, 2, 0); opt {
	case "collect":
		runtime.GC()
		l.PushInteger(0)
	case "step":
		runtime.GC()
		l.PushBoolean(true)
	case "count":
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		l.PushNumber(float64(stats.HeapAlloc >> 10))
		l.PushInteger(int(stats.HeapAlloc & 0x3ff))
		return 2
	default:
		l.PushInteger(-1)
	}
	return 1
}

func _metaTable(l State) int {
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

func pairs(method string, isZero bool, iter Function) Function {
	return func(l State) int {
		if !MetaField(l, 1, method) { // no metamethod?
			CheckType(l, 1, TypeTable) // argument must be a table
			l.PushGoFunction(iter)     // will return generator,
			l.PushValue(1)             // state,
			if isZero {                // and initial value
				l.PushInteger(0)
			} else {
				l.PushNil()
			}
		} else {
			l.PushValue(1) // argument 'self' to metamethod
			l.Call(1, 3)   // get 3 values from metamethod
		}
		return 3
	}
}

func intPairs(l State) int {
	i := CheckInteger(l, 2)
	CheckType(l, 1, TypeTable)
	i++ // next value
	l.PushInteger(i)
	l.RawGetInt(1, i)
	if l.IsNil(-1) {
		return 1
	}
	return 2
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

func _finishProtectedCall(l State, status bool) int {
	if !l.CheckStack(1) {
		l.SetTop(0) // create space for return values
		l.PushBoolean(false)
		l.PushString("stack overflow")
		return 2 // return false, message
	}
	l.PushBoolean(status) // first result (status)
	l.Replace(1)          // put first result in the first slot
	return l.Top()
}

func _protectedCallContinuation(l State) int {
	s, _ := l.Context()
	return _finishProtectedCall(l, s == Yield)
}

func _protectedCall(l State) int {
	CheckAny(l, 1)
	l.PushNil()
	l.Insert(1) // create space for status result
	return _finishProtectedCall(l, Ok == l.ProtectedCallWithContinuation(l.Top()-2, MultipleReturns, 0, 0, _protectedCallContinuation))
}

func _protectedCallX(l State) int {
	n := l.Top()
	ArgumentCheck(l, n >= 2, 2, "value expected")
	l.PushValue(1) // exchange function and error handler
	l.Copy(2, 1)
	l.Replace(2)
	return _finishProtectedCall(l, Ok == l.ProtectedCallWithContinuation(n-2, MultipleReturns, 1, 0, _protectedCallContinuation))
}

func _toString(l State) int {
	CheckAny(l, 1)
	ToString(l, 1)
	return 1
}

var baseFunctions = []RegistryFunction{
	{"assert", _assert},
	{"collectgarbage", _collectGarbage},
	// {"dofile", _doFile},
	{"error", _error},
	{"getmetatable", _metaTable},
	{"ipairs", pairs("__ipairs", true, intPairs)},
	// {"loadfile", _loadFile},
	// {"load", _load},
	{"next", _next},
	{"pairs", pairs("__pairs", false, _next)},
	{"pcall", _protectedCall},
	{"print", _print},
	{"rawequal", _rawEqual},
	{"rawlen", _rawLength},
	{"rawget", _rawGet},
	{"rawset", _rawSet},
	{"select", _select},
	{"setmetatable", _setMetaTable},
	{"tonumber", _toNumber},
	{"tostring", _toString},
	{"type", _type},
	{"xpcall", _protectedCallX},
}

func OpenBase(l State) {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	SetFunctions(l, baseFunctions, 0)
	l.PushString(Version)
	l.SetField(-2, "_VERSION")
}
