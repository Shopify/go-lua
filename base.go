package lua

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func next(l *State) int {
	CheckType(l, 1, TypeTable)
	l.SetTop(2)
	if l.Next(1) {
		return 2
	}
	l.PushNil()
	return 1
}

func pairs(method string, isZero bool, iter Function) Function {
	return func(l *State) int {
		if hasMetamethod := MetaField(l, 1, method); !hasMetamethod {
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

func intPairs(l *State) int {
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

func finishProtectedCall(l *State, status bool) int {
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

func protectedCallContinuation(l *State) int {
	_, shouldYield, _ := l.Context()
	return finishProtectedCall(l, shouldYield)
}

func loadHelper(l *State, s error, e int) int {
	if s == nil {
		if e != 0 {
			l.PushValue(e)
			if _, ok := SetUpValue(l, -2, 1); !ok {
				l.Pop(1)
			}
		}
		return 1
	}
	l.PushNil()
	l.Insert(-2)
	return 2
}

type genericReader struct {
	l *State
	r *strings.Reader
	e error
}

func (r *genericReader) Read(b []byte) (n int, err error) {
	if r.e != nil {
		return 0, r.e
	}
	if l := r.l; r.r == nil {
		CheckStackWithMessage(l, 2, "too many nested functions")
		l.PushValue(1)
		if l.Call(0, 1); l.IsNil(-1) {
			l.Pop(1)
			return 0, io.EOF
		} else if !l.IsString(-1) {
			Errorf(l, "reader function must return a string")
		}
		if s, ok := l.ToString(-1); ok {
			r.r = strings.NewReader(s)
		} else {
			return 0, io.EOF
		}
	}
	if n, err = r.r.Read(b); err == io.EOF {
		r.r, err = nil, nil
	} else if err != nil {
		r.e = err
	}
	return
}

var baseLibrary = []RegistryFunction{
	{"assert", func(l *State) int {
		if !l.ToBoolean(1) {
			Errorf(l, "%s", OptString(l, 2, "assertion failed!"))
			panic("unreachable")
		}
		return l.Top()
	}},
	{"collectgarbage", func(l *State) int {
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
	}},
	{"dofile", func(l *State) int {
		f := OptString(l, 1, "")
		if l.SetTop(1); LoadFile(l, f, "") != nil {
			l.Error()
			panic("unreachable")
		}
		continuation := func(l *State) int { return l.Top() - 1 }
		l.CallWithContinuation(0, MultipleReturns, 0, continuation)
		return continuation(l)
	}},
	{"error", func(l *State) int {
		level := OptInteger(l, 2, 1)
		l.SetTop(1)
		if l.IsString(1) && level > 0 {
			Where(l, level)
			l.PushValue(1)
			l.Concat(2)
		}
		l.Error()
		panic("unreachable")
	}},
	{"getmetatable", func(l *State) int {
		CheckAny(l, 1)
		if !l.MetaTable(1) {
			l.PushNil()
			return 1
		}
		MetaField(l, 1, "__metatable")
		return 1
	}},
	{"ipairs", pairs("__ipairs", true, intPairs)},
	{"loadfile", func(l *State) int {
		f, m, e := OptString(l, 1, ""), OptString(l, 2, ""), 3
		if l.IsNone(e) {
			e = 0
		}
		return loadHelper(l, LoadFile(l, f, m), e)
	}},
	{"load", func(l *State) int {
		m, e := OptString(l, 3, "bt"), 4
		if l.IsNone(e) {
			e = 0
		}
		var err error
		if s, ok := l.ToString(1); ok {
			err = LoadBuffer(l, s, OptString(l, 2, s), m)
		} else {
			chunkName := OptString(l, 2, "=(load)")
			CheckType(l, 1, TypeFunction)
			err = l.Load(&genericReader{l: l}, chunkName, m)
		}
		return loadHelper(l, err, e)
	}},
	{"next", next},
	{"pairs", pairs("__pairs", false, next)},
	{"pcall", func(l *State) int {
		CheckAny(l, 1)
		l.PushNil()
		l.Insert(1) // create space for status result
		return finishProtectedCall(l, nil == l.ProtectedCallWithContinuation(l.Top()-2, MultipleReturns, 0, 0, protectedCallContinuation))
	}},
	{"print", func(l *State) int {
		n := l.Top()
		l.Global("tostring")
		for i := 1; i <= n; i++ {
			l.PushValue(-1) // function to be called
			l.PushValue(i)  // value to print
			l.Call(1, 1)
			s, ok := l.ToString(-1)
			if !ok {
				Errorf(l, "'tostring' must return a string to 'print'")
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
	}},
	{"rawequal", func(l *State) int {
		CheckAny(l, 1)
		CheckAny(l, 2)
		l.PushBoolean(l.RawEqual(1, 2))
		return 1
	}},
	{"rawlen", func(l *State) int {
		t := l.TypeOf(1)
		ArgumentCheck(l, t == TypeTable || t == TypeString, 1, "table or string expected")
		l.PushInteger(l.RawLength(1))
		return 1
	}},
	{"rawget", func(l *State) int {
		CheckType(l, 1, TypeTable)
		CheckAny(l, 2)
		l.SetTop(2)
		l.RawGet(1)
		return 1
	}},
	{"rawset", func(l *State) int {
		CheckType(l, 1, TypeTable)
		CheckAny(l, 2)
		CheckAny(l, 3)
		l.SetTop(3)
		l.RawSet(1)
		return 1
	}},
	{"select", func(l *State) int {
		n := l.Top()
		if l.TypeOf(1) == TypeString {
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
	}},
	{"setmetatable", func(l *State) int {
		t := l.TypeOf(2)
		CheckType(l, 1, TypeTable)
		ArgumentCheck(l, t == TypeNil || t == TypeTable, 2, "nil or table expected")
		if MetaField(l, 1, "__metatable") {
			Errorf(l, "cannot change a protected metatable")
		}
		l.SetTop(2)
		l.SetMetaTable(1)
		return 1
	}},
	{"tonumber", func(l *State) int {
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
			if i, err := strconv.ParseInt(strings.TrimSpace(s), base, 64); err == nil {
				l.PushNumber(float64(i))
				return 1
			}
		}
		l.PushNil()
		return 1
	}},
	{"tostring", func(l *State) int {
		CheckAny(l, 1)
		ToStringMeta(l, 1)
		return 1
	}},
	{"type", func(l *State) int {
		CheckAny(l, 1)
		l.PushString(TypeNameOf(l, 1))
		return 1
	}},
	{"xpcall", func(l *State) int {
		n := l.Top()
		ArgumentCheck(l, n >= 2, 2, "value expected")
		l.PushValue(1) // exchange function and error handler
		l.Copy(2, 1)
		l.Replace(2)
		return finishProtectedCall(l, nil == l.ProtectedCallWithContinuation(n-2, MultipleReturns, 1, 0, protectedCallContinuation))
	}},
}

// BaseOpen opens the basic library. Usually passed to Require.
func BaseOpen(l *State) int {
	l.PushGlobalTable()
	l.PushGlobalTable()
	l.SetField(-2, "_G")
	SetFunctions(l, baseLibrary, 0)
	l.PushString(VersionString)
	l.SetField(-2, "_VERSION")
	return 1
}
