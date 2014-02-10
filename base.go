package lua

import (
	"os"
	"runtime"
	"strconv"
)

func next(l *State) int {
	CheckType(l, 1, TypeTable)
	SetTop(l, 2)
	if Next(l, 1) {
		return 2
	}
	PushNil(l)
	return 1
}

func pairs(method string, isZero bool, iter Function) Function {
	return func(l *State) int {
		if !MetaField(l, 1, method) { // no metamethod?
			CheckType(l, 1, TypeTable) // argument must be a table
			PushGoFunction(l, iter)    // will return generator,
			PushValue(l, 1)            // state,
			if isZero {                // and initial value
				PushInteger(l, 0)
			} else {
				PushNil(l)
			}
		} else {
			PushValue(l, 1) // argument 'self' to metamethod
			Call(l, 1, 3)   // get 3 values from metamethod
		}
		return 3
	}
}

func intPairs(l *State) int {
	i := CheckInteger(l, 2)
	CheckType(l, 1, TypeTable)
	i++ // next value
	PushInteger(l, i)
	RawGetInt(l, 1, i)
	if IsNil(l, -1) {
		return 1
	}
	return 2
}

func finishProtectedCall(l *State, status bool) int {
	if !CheckStack(l, 1) {
		SetTop(l, 0) // create space for return values
		PushBoolean(l, false)
		PushString(l, "stack overflow")
		return 2 // return false, message
	}
	PushBoolean(l, status) // first result (status)
	Replace(l, 1)          // put first result in the first slot
	return Top(l)
}

func protectedCallContinuation(l *State) int {
	_, shouldYield, _ := Context(l)
	return finishProtectedCall(l, shouldYield)
}

func loadHelper(l *State, s error, e int) int {
	if s == nil {
		if e != 0 {
			PushValue(l, e)
			if _, ok := SetUpValue(l, -2, 1); !ok {
				Pop(l, 1)
			}
		}
		return 1
	}
	PushNil(l)
	Insert(l, -2)
	return 2
}

var baseLibrary = []RegistryFunction{
	{"assert", func(l *State) int {
		if !ToBoolean(l, 1) {
			Errorf(l, "%s", OptString(l, 2, "assertion failed!"))
			panic("unreachable")
		}
		return Top(l)
	}},
	{"collectgarbage", func(l *State) int {
		switch opt, _ := OptString(l, 1, "collect"), OptInteger(l, 2, 0); opt {
		case "collect":
			runtime.GC()
			PushInteger(l, 0)
		case "step":
			runtime.GC()
			PushBoolean(l, true)
		case "count":
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			PushNumber(l, float64(stats.HeapAlloc>>10))
			PushInteger(l, int(stats.HeapAlloc&0x3ff))
			return 2
		default:
			PushInteger(l, -1)
		}
		return 1
	}},
	{"dofile", func(l *State) int {
		f := OptString(l, 1, "")
		if SetTop(l, 1); LoadFile(l, f, "") != nil {
			Error(l)
			panic("unreachable")
		}
		continuation := func(l *State) int { return Top(l) - 1 }
		CallWithContinuation(l, 0, MultipleReturns, 0, continuation)
		return continuation(l)
	}},
	{"error", func(l *State) int {
		level := OptInteger(l, 2, 1)
		SetTop(l, 1)
		if IsString(l, 1) && level > 0 {
			Where(l, level)
			PushValue(l, 1)
			Concat(l, 2)
		}
		Error(l)
		panic("unreachable")
	}},
	{"getmetatable", func(l *State) int {
		CheckAny(l, 1)
		if !MetaTable(l, 1) {
			PushNil(l)
			return 1
		}
		MetaField(l, 1, "__metatable")
		return 1
	}},
	{"ipairs", pairs("__ipairs", true, intPairs)},
	{"loadfile", func(l *State) int {
		f, m, e := OptString(l, 1, ""), OptString(l, 2, ""), 3
		if IsNone(l, e) {
			e = 0
		}
		return loadHelper(l, LoadFile(l, f, m), e)
	}},
	// {"load", load},
	{"next", next},
	{"pairs", pairs("__pairs", false, next)},
	{"pcall", func(l *State) int {
		CheckAny(l, 1)
		PushNil(l)
		Insert(l, 1) // create space for status result
		return finishProtectedCall(l, nil == ProtectedCallWithContinuation(l, Top(l)-2, MultipleReturns, 0, 0, protectedCallContinuation))
	}},
	{"print", func(l *State) int {
		n := Top(l)
		Global(l, "tostring")
		for i := 1; i <= n; i++ {
			PushValue(l, -1) // function to be called
			PushValue(l, i)  // value to print
			Call(l, 1, 1)
			s, ok := ToString(l, -1)
			if !ok {
				Errorf(l, "'tostring' must return a string to 'print'")
				panic("unreachable")
			}
			if i > 1 {
				os.Stdout.WriteString("\t")
			}
			os.Stdout.WriteString(s)
			Pop(l, 1) // pop result
		}
		os.Stdout.WriteString("\n")
		os.Stdout.Sync()
		return 0
	}},
	{"rawequal", func(l *State) int {
		CheckAny(l, 1)
		CheckAny(l, 2)
		ok := RawEqual(l, 1, 2)
		PushBoolean(l, ok)
		return 1
	}},
	{"rawlen", func(l *State) int {
		t := TypeOf(l, 1)
		ArgumentCheck(l, t == TypeTable || t == TypeString, 1, "table or string expected")
		PushInteger(l, RawLength(l, 1))
		return 1
	}},
	{"rawget", func(l *State) int {
		CheckType(l, 1, TypeTable)
		CheckAny(l, 2)
		SetTop(l, 2)
		RawGet(l, 1)
		return 1
	}},
	{"rawset", func(l *State) int {
		CheckType(l, 1, TypeTable)
		CheckAny(l, 2)
		CheckAny(l, 3)
		SetTop(l, 3)
		RawSet(l, 1)
		return 1
	}},
	{"select", func(l *State) int {
		n := Top(l)
		if TypeOf(l, 1) == TypeString {
			if s, _ := ToString(l, 1); s[0] == '#' {
				PushInteger(l, n-1)
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
	}},
	{"setmetatable", func(l *State) int {
		t := TypeOf(l, 2)
		CheckType(l, 1, TypeTable)
		ArgumentCheck(l, t == TypeNil || t == TypeTable, 2, "nil or table expected")
		if MetaField(l, 1, "__metatable") {
			Errorf(l, "cannot change a protected metatable")
		}
		SetTop(l, 2)
		SetMetaTable(l, 1)
		return 1
	}},
	{"tonumber", func(l *State) int {
		if IsNoneOrNil(l, 2) { // standard conversion
			if n, ok := ToNumber(l, 1); ok {
				PushNumber(l, n)
				return 1
			}
			CheckAny(l, 1)
		} else {
			s := CheckString(l, 1)
			base := CheckInteger(l, 2)
			ArgumentCheck(l, 2 <= base && base <= 36, 2, "base out of range")
			if i, err := strconv.ParseInt(s, base, 64); err == nil { // TODO strings.TrimSpace(s)?
				PushNumber(l, float64(i))
				return 1
			}
		}
		PushNil(l)
		return 1
	}},
	{"tostring", func(l *State) int {
		CheckAny(l, 1)
		ToString(l, 1)
		return 1
	}},
	{"type", func(l *State) int {
		CheckAny(l, 1)
		PushString(l, TypeName(l, 1))
		return 1
	}},
	{"xpcall", func(l *State) int {
		n := Top(l)
		ArgumentCheck(l, n >= 2, 2, "value expected")
		PushValue(l, 1) // exchange function and error handler
		Copy(l, 2, 1)
		Replace(l, 2)
		return finishProtectedCall(l, nil == ProtectedCallWithContinuation(l, n-2, MultipleReturns, 1, 0, protectedCallContinuation))
	}},
}

func BaseOpen(l *State) int {
	PushGlobalTable(l)
	PushGlobalTable(l)
	SetField(l, -2, "_G")
	SetFunctions(l, baseLibrary, 0)
	PushString(l, VersionString)
	SetField(l, -2, "_VERSION")
	return 1
}
