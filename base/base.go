package base

import (
	"github.com/Shopify/go-lua"
	"os"
	"runtime"
	"strconv"
)

func print(l *lua.State) int {
	n := lua.Top(l)
	lua.Global(l, "tostring")
	for i := 1; i <= n; i++ {
		lua.PushValue(l, -1) // function to be called
		lua.PushValue(l, i)  // value to print
		lua.Call(l, 1, 1)
		s, ok := lua.ToString(l, -1)
		if !ok {
			lua.Errorf(l, "'tostring' must return a string to 'print'")
			panic("unreachable")
		}
		if i > 1 {
			os.Stdout.WriteString("\t")
		}
		os.Stdout.WriteString(s)
		lua.Pop(l, 1) // pop result
	}
	os.Stdout.WriteString("\n")
	os.Stdout.Sync()
	return 0
}

func toNumber(l *lua.State) int {
	if lua.IsNoneOrNil(l, 2) { // standard conversion
		if n, ok := lua.ToNumber(l, 1); ok {
			lua.PushNumber(l, n)
			return 1
		}
		lua.CheckAny(l, 1)
	} else {
		s := lua.CheckString(l, 1)
		base := lua.CheckInteger(l, 2)
		lua.ArgumentCheck(l, 2 <= base && base <= 36, 2, "base out of range")
		if i, err := strconv.ParseInt(s, base, 64); err == nil { // TODO strings.TrimSpace(s)?
			lua.PushNumber(l, float64(i))
			return 1
		}
	}
	lua.PushNil(l)
	return 1
}

func error(l *lua.State) int {
	level := lua.OptInteger(l, 2, 1)
	lua.SetTop(l, 1)
	if lua.IsString(l, 1) && level > 0 {
		lua.Where(l, level)
		lua.PushValue(l, 1)
		lua.Concat(l, 2)
	}
	lua.Error(l)
	panic("unreachable")
}

func collectGarbage(l *lua.State) int {
	switch opt, _ := lua.OptString(l, 1, "collect"), lua.OptInteger(l, 2, 0); opt {
	case "collect":
		runtime.GC()
		lua.PushInteger(l, 0)
	case "step":
		runtime.GC()
		lua.PushBoolean(l, true)
	case "count":
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		lua.PushNumber(l, float64(stats.HeapAlloc>>10))
		lua.PushInteger(l, int(stats.HeapAlloc&0x3ff))
		return 2
	default:
		lua.PushInteger(l, -1)
	}
	return 1
}

func metaTable(l *lua.State) int {
	lua.CheckAny(l, 1)
	if !lua.MetaTable(l, 1) {
		lua.PushNil(l)
		return 1
	}
	lua.MetaField(l, 1, "__metatable")
	return 1
}

func setMetaTable(l *lua.State) int {
	t := lua.Type(l, 2)
	lua.CheckType(l, 1, lua.TypeTable)
	lua.ArgumentCheck(l, t == lua.TypeNil || t == lua.TypeTable, 2, "nil or table expected")
	if lua.MetaField(l, 1, "__metatable") {
		lua.Errorf(l, "cannot change a protected metatable")
	}
	lua.SetTop(l, 2)
	lua.SetMetaTable(l, 1)
	return 1
}

func rawEqual(l *lua.State) int {
	lua.CheckAny(l, 1)
	lua.CheckAny(l, 2)
	lua.PushBoolean(l, lua.RawEqual(l, 1, 2))
	return 1
}

func rawLength(l *lua.State) int {
	t := lua.Type(l, 1)
	lua.ArgumentCheck(l, t == lua.TypeTable || t == lua.TypeString, 1, "table or string expected")
	lua.PushInteger(l, lua.RawLength(l, 1))
	return 1
}

func rawGet(l *lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	lua.CheckAny(l, 2)
	lua.SetTop(l, 2)
	lua.RawGet(l, 1)
	return 1
}

func rawSet(l *lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	lua.CheckAny(l, 2)
	lua.CheckAny(l, 3)
	lua.SetTop(l, 3)
	lua.RawSet(l, 1)
	return 1
}

func _type(l *lua.State) int {
	lua.CheckAny(l, 1)
	lua.PushString(l, lua.TypeName(l, 1))
	return 1
}

func next(l *lua.State) int {
	lua.CheckType(l, 1, lua.TypeTable)
	lua.SetTop(l, 2)
	if lua.Next(l, 1) {
		return 2
	}
	lua.PushNil(l)
	return 1
}

func pairs(method string, isZero bool, iter lua.Function) lua.Function {
	return func(l *lua.State) int {
		if !lua.MetaField(l, 1, method) { // no metamethod?
			lua.CheckType(l, 1, lua.TypeTable) // argument must be a table
			lua.PushGoFunction(l, iter)        // will return generator,
			lua.PushValue(l, 1)                // state,
			if isZero {                        // and initial value
				lua.PushInteger(l, 0)
			} else {
				lua.PushNil(l)
			}
		} else {
			lua.PushValue(l, 1) // argument 'self' to metamethod
			lua.Call(l, 1, 3)   // get 3 values from metamethod
		}
		return 3
	}
}

func intPairs(l *lua.State) int {
	i := lua.CheckInteger(l, 2)
	lua.CheckType(l, 1, lua.TypeTable)
	i++ // next value
	lua.PushInteger(l, i)
	lua.RawGetInt(l, 1, i)
	if lua.IsNil(l, -1) {
		return 1
	}
	return 2
}

func assert(l *lua.State) int {
	if !lua.ToBoolean(l, 1) {
		lua.Errorf(l, "%s", lua.OptString(l, 2, "assertion failed!"))
		panic("unreachable")
	}
	return lua.Top(l)
}

func _select(l *lua.State) int {
	n := lua.Top(l)
	if lua.Type(l, 1) == lua.TypeString {
		if s, _ := lua.ToString(l, 1); s[0] == '#' {
			lua.PushInteger(l, n-1)
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

func finishProtectedCall(l *lua.State, status bool) int {
	if !lua.CheckStack(l, 1) {
		lua.SetTop(l, 0) // create space for return values
		lua.PushBoolean(l, false)
		lua.PushString(l, "stack overflow")
		return 2 // return false, message
	}
	lua.PushBoolean(l, status) // first result (status)
	lua.Replace(l, 1)          // put first result in the first slot
	return lua.Top(l)
}

func protectedCallContinuation(l *lua.State) int {
	s, _ := lua.Context(l)
	return finishProtectedCall(l, s == lua.Yield)
}

func protectedCall(l *lua.State) int {
	lua.CheckAny(l, 1)
	lua.PushNil(l)
	lua.Insert(l, 1) // create space for status result
	return finishProtectedCall(l, lua.Ok == lua.ProtectedCallWithContinuation(l, lua.Top(l)-2, lua.MultipleReturns, 0, 0, protectedCallContinuation))
}

func protectedCallX(l *lua.State) int {
	n := lua.Top(l)
	lua.ArgumentCheck(l, n >= 2, 2, "value expected")
	lua.PushValue(l, 1) // exchange function and error handler
	lua.Copy(l, 2, 1)
	lua.Replace(l, 2)
	return finishProtectedCall(l, lua.Ok == lua.ProtectedCallWithContinuation(l, n-2, lua.MultipleReturns, 1, 0, protectedCallContinuation))
}

func toString(l *lua.State) int {
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

func Open(l *lua.State) int {
	lua.PushGlobalTable(l)
	lua.PushGlobalTable(l)
	lua.SetField(l, -2, "_G")
	lua.SetFunctions(l, baseFunctions, 0)
	lua.PushString(l, lua.VersionString)
	lua.SetField(l, -2, "_VERSION")
	return 1
}
