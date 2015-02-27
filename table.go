package lua

import (
	"fmt"
	"sort"
)

type sortHelper struct {
	l           *State
	n           int
	hasFunction bool
}

func (h sortHelper) Len() int { return h.n }

func (h sortHelper) Swap(i, j int) {
	// Convert Go to Lua indices
	i++
	j++
	h.l.RawGetInt(1, i)
	h.l.RawGetInt(1, j)
	h.l.RawSetInt(1, i)
	h.l.RawSetInt(1, j)
}

func (h sortHelper) Less(i, j int) bool {
	// Convert Go to Lua indices
	i++
	j++
	if h.hasFunction {
		h.l.PushValue(2)
		h.l.RawGetInt(1, i)
		h.l.RawGetInt(1, j)
		h.l.Call(2, 1)
		b := h.l.ToBoolean(-1)
		h.l.Pop(1)
		return b
	}
	h.l.RawGetInt(1, i)
	h.l.RawGetInt(1, j)
	b := h.l.Compare(-2, -1, OpLT)
	h.l.Pop(2)
	return b
}

var tableLibrary = []RegistryFunction{
	{"concat", func(l *State) int {
		CheckType(l, 1, TypeTable)
		sep := OptString(l, 2, "")
		i := OptInteger(l, 3, 1)
		var last int
		if l.IsNoneOrNil(4) {
			last = LengthEx(l, 1)
		} else {
			last = CheckInteger(l, 4)
		}
		s := ""
		addField := func() {
			l.RawGetInt(1, i)
			if str, ok := l.ToString(-1); ok {
				s += str
			} else {
				Errorf(l, fmt.Sprintf("invalid value (%s) at index %d in table for 'concat'", TypeNameOf(l, -1), i))
			}
			l.Pop(1)
		}
		for ; i < last; i++ {
			addField()
			s += sep
		}
		if i == last {
			addField()
		}
		l.PushString(s)
		return 1
	}},
	{"insert", func(l *State) int {
		CheckType(l, 1, TypeTable)
		e := LengthEx(l, 1) + 1 // First empty element.
		switch l.Top() {
		case 2:
			l.RawSetInt(1, e) // Insert new element at the end.
		case 3:
			pos := CheckInteger(l, 2)
			ArgumentCheck(l, 1 <= pos && pos <= e, 2, "position out of bounds")
			for i := e; i > pos; i-- {
				l.RawGetInt(1, i-1)
				l.RawSetInt(1, i) // t[i] = t[i-1]
			}
			l.RawSetInt(1, pos) // t[pos] = v
		default:
			Errorf(l, "wrong number of arguments to 'insert'")
		}
		return 0
	}},
	{"pack", func(l *State) int {
		n := l.Top()
		l.CreateTable(n, 1)
		l.PushInteger(n)
		l.SetField(-2, "n")
		if n > 0 {
			l.PushValue(1)
			l.RawSetInt(-2, 1)
			l.Replace(1)
			for i := n; i >= 2; i-- {
				l.RawSetInt(1, i)
			}
		}
		return 1
	}},
	{"unpack", func(l *State) int {
		CheckType(l, 1, TypeTable)
		i := OptInteger(l, 2, 1)
		var e int
		if l.IsNoneOrNil(3) {
			e = LengthEx(l, 1)
		} else {
			e = CheckInteger(l, 3)
		}
		if i > e {
			return 0
		}
		n := e - i + 1
		if n <= 0 || !l.CheckStack(n) {
			Errorf(l, "too many results to unpack")
			panic("unreachable")
		}
		for l.RawGetInt(1, i); i < e; i++ {
			l.RawGetInt(1, i+1)
		}
		return n
	}},
	{"remove", func(l *State) int {
		CheckType(l, 1, TypeTable)
		size := LengthEx(l, 1)
		pos := OptInteger(l, 2, size)
		if pos != size {
			ArgumentCheck(l, 1 <= pos && pos <= size+1, 2, "position out of bounds")
		}
		for l.RawGetInt(1, pos); pos < size; pos++ {
			l.RawGetInt(1, pos+1)
			l.RawSetInt(1, pos) // t[pos] = t[pos+1]
		}
		l.PushNil()
		l.RawSetInt(1, pos) // t[pos] = nil
		return 1
	}},
	{"sort", func(l *State) int {
		CheckType(l, 1, TypeTable)
		n := LengthEx(l, 1)
		hasFunction := !l.IsNoneOrNil(2)
		if hasFunction {
			CheckType(l, 2, TypeFunction)
		}
		l.SetTop(2)
		h := sortHelper{l, n, hasFunction}
		sort.Sort(h)
		// Check result is sorted.
		if n > 0 && h.Less(n-1, 0) {
			Errorf(l, "invalid order function for sorting")
		}
		return 0
	}},
}

// TableOpen opens the table library. Usually passed to Require.
func TableOpen(l *State) int {
	NewLibrary(l, tableLibrary)
	return 1
}
