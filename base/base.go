package base

import (
	"github.com/Shopify/go-lua"
	"os"
	"runtime"
	"strconv"
)

func print(l lua.State) int {
	n := l.Top()
	l.Global("tostring")
	for i := 1; i <= n; i++ {
		l.PushValue(-1) // function to be called
		l.PushValue(i)  // value to print
		l.Call(1, 1)
		s, ok := l.ToString(-1)
		if !ok {
			lua.Error(l, "'tostring' must return a string to 'print'")
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

func toNumber(l lua.State) int {
	if l.IsNoneOrNil(2) { // standard conversion
		if n, ok := l.ToNumber(1); ok {
			l.PushNumber(n)
			return 1
		}
		lua.CheckAny(l, 1)
	} else {
		s := lua.CheckString(l, 1)
		base := lua.CheckInteger(l, 2)
		lua.ArgumentCheck(l, 2 <= base && base <= 36, 2, "base out of range")
		if i, err := strconv.ParseInt(s, base, 64); err == nil { // TODO strings.TrimSpace(s)?
			l.PushNumber(float64(i))
			return 1
		}
	}
	l.PushNil()
	return 1
}

func error(l lua.State) int {
	level := lua.OptInteger(l, 2, 1)
	l.SetTop(1)
	if l.IsString(1) && level > 0 {
		lua.Where(l, level)
		l.PushValue(1)
		l.Concat(2)
	}
	l.Error()
	panic("unreachable")
}

func collectGarbage(l lua.State) int {
	switch opt, _ := lua.OptString(l, 1, "collect"), lua.OptInteger(l, 2, 0); opt {
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

func metaTable(l lua.State) int {
	lua.CheckAny(l, 1)
	if !l.MetaTable(1) {
		l.PushNil()
		return 1
	}
	lua.MetaField(l, 1, "__metatable")
	return 1
}

func setMetaTable(l lua.State) int {
	t := l.Type(2)
	lua.CheckType(l, 1, lua.TypeTable)
	lua.ArgumentCheck(l, t == lua.TypeNil || t == lua.TypeTable, 2, "nil or table expected")
	if lua.MetaField(l, 1, "__metatable") {
		lua.Error(l, "cannot change a protected metatable")
	}
	l.SetTop(2)
	l.SetMetaTable(1)
	return 1
}

func rawEqual(l lua.State) int {
	lua.CheckAny(l, 1)
	lua.CheckAny(l, 2)
	l.PushBoolean(l.RawEqual(1, 2))
	return 1
}

func rawLength(l lua.State) int {
	t := l.Type(1)
	lua.ArgumentCheck(l, t == lua.TypeTable || t == lua.TypeString, 1, "table or string expected")
	l.PushInteger(l.RawLength(1))
	return 1
}

func rawGet(l lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	lua.CheckAny(l, 2)
	l.SetTop(2)
	l.RawGet(1)
	return 1
}

func rawSet(l lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	lua.CheckAny(l, 2)
	lua.CheckAny(l, 3)
	l.SetTop(3)
	l.RawSet(1)
	return 1
}

func _type(l lua.State) int {
	lua.CheckAny(l, 1)
	l.PushString(l.TypeName(1))
	return 1
}

func next(l lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	l.SetTop(2)
	if l.Next(1) {
		return 2
	}
	l.PushNil()
	return 1
}

func pairs(method string, isZero bool, iter lua.Function) lua.Function {
	return func(l lua.State) int {
		if !lua.MetaField(l, 1, method) { // no metamethod?
			lua.CheckType(l, 1, lua.TypeTable) // argument must be a table
			l.PushGoFunction(iter)             // will return generator,
			l.PushValue(1)                     // state,
			if isZero {                        // and initial value
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

func intPairs(l lua.State) int {
	i := lua.CheckInteger(l, 2)
	lua.CheckType(l, 1, lua.TypeTable)
	i++ // next value
	l.PushInteger(i)
	l.RawGetInt(1, i)
	if l.IsNil(-1) {
		return 1
	}
	return 2
}

func assert(l lua.State) int {
	if !l.ToBoolean(1) {
		lua.Error(l, "%s", lua.OptString(l, 2, "assertion failed!"))
		panic("unreachable")
	}
	return l.Top()
}

func _select(l lua.State) int {
	n := l.Top()
	if l.Type(1) == lua.TypeString {
		if s, _ := l.ToString(1); s[0] == '#' {
			l.PushInteger(n - 1)
			return 1
		}
	}
	i := lua.CheckInteger(l, 1)
	if i < 0 {
		i = n + i
	} else if i > n {
		i = n
	}
	lua.ArgumentCheck(l, 1 <= i, 1, "index out of range")
	return n - i
}

func finishProtectedCall(l lua.State, status bool) int {
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

func protectedCallContinuation(l lua.State) int {
	s, _ := l.Context()
	return finishProtectedCall(l, s == lua.Yield)
}

func protectedCall(l lua.State) int {
	lua.CheckAny(l, 1)
	l.PushNil()
	l.Insert(1) // create space for status result
	return finishProtectedCall(l, lua.Ok == l.ProtectedCallWithContinuation(l.Top()-2, lua.MultipleReturns, 0, 0, protectedCallContinuation))
}

func protectedCallX(l lua.State) int {
	n := l.Top()
	lua.ArgumentCheck(l, n >= 2, 2, "value expected")
	l.PushValue(1) // exchange function and error handler
	l.Copy(2, 1)
	l.Replace(2)
	return finishProtectedCall(l, lua.Ok == l.ProtectedCallWithContinuation(n-2, lua.MultipleReturns, 1, 0, protectedCallContinuation))
}

func toString(l lua.State) int {
	lua.CheckAny(l, 1)
	lua.ToString(l, 1)
	return 1
}

var baseFunctions = []lua.RegistryFunction{
	{"assert", assert},
	{"collectgarbage", collectGarbage},
	// {"dofile", doFile},
	{"error", error},
	{"getmetatable", metaTable},
	{"ipairs", pairs("__ipairs", true, intPairs)},
	// {"loadfile", loadFile},
	// {"load", load},
	{"next", next},
	{"pairs", pairs("__pairs", false, next)},
	{"pcall", protectedCall},
	{"print", print},
	{"rawequal", rawEqual},
	{"rawlen", rawLength},
	{"rawget", rawGet},
	{"rawset", rawSet},
	{"select", _select},
	{"setmetatable", setMetaTable},
	{"tonumber", toNumber},
	{"tostring", toString},
	{"type", _type},
	{"xpcall", protectedCallX},
}

func Open(l lua.State) int {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	lua.SetFunctions(l, baseFunctions, 0)
	l.PushString(lua.Version)
	l.SetField(-2, "_VERSION")
	return 1
}
