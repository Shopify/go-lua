package lua

import (
	"fmt"
)

var tableLibrary = []RegistryFunction{
	{"concat", func(l *State) int {
		CheckType(l, 1, TypeTable)
		sep := OptString(l, 2, "")
		i := OptInteger(l, 3, 1)
		var last int
		if IsNoneOrNil(l, 4) {
			last = LengthEx(l, 1)
		} else {
			last = CheckInteger(l, 4)
		}
		s := ""
		addField := func() {
			RawGetInt(l, 1, i)
			if str, ok := ToString(l, -1); ok {
				s += str
			} else {
				Errorf(l, fmt.Sprintf("invalid value (%s) at index %d in table for 'concat'", TypeNameOf(l, -1), i))
			}
		}
		for ; i < last; i++ {
			addField()
			s += sep
		}
		if i == last {
			addField()
		}
		PushString(l, s)
		return 1
	}},
	{"insert", func(l *State) int {
		CheckType(l, 1, TypeTable)
		e := LengthEx(l, 1) + 1 // First empty element.
		switch Top(l) {
		case 2:
			RawSetInt(l, 1, e) // Insert new element at the end.
		case 3:
			pos := CheckInteger(l, 2)
			ArgumentCheck(l, 1 <= pos && pos <= e, 2, "position out of bounds")
			for i := e; i > pos; i-- {
				RawGetInt(l, 1, i-1)
				RawSetInt(l, 1, i) // t[i] = t[i-1]
			}
			RawSetInt(l, 1, pos) // t[pos] = v
		default:
			Errorf(l, "wrong number of arguments to 'insert'")
		}
		return 0
	}},
	{"pack", func(l *State) int {
		n := Top(l)
		CreateTable(l, n, 1)
		PushInteger(l, n)
		SetField(l, -2, "n")
		if n > 0 {
			PushValue(l, 1)
			RawSetInt(l, -2, 1)
			Replace(l, 1)
			for i := n; i >= 2; i-- {
				RawSetInt(l, 1, i)
			}
		}
		return 1
	}},
	{"unpack", func(l *State) int {
		CheckType(l, 1, TypeTable)
		i := OptInteger(l, 2, 1)
		var e int
		if IsNoneOrNil(l, 3) {
			e = LengthEx(l, 1)
		} else {
			e = CheckInteger(l, 3)
		}
		if i > e {
			return 0
		}
		n := e - i + 1
		if n <= 0 || !CheckStack(l, n) {
			Errorf(l, "too many results to unpack")
			panic("unreachable")
		}
		for RawGetInt(l, 1, i); i < e; i++ {
			RawGetInt(l, 1, i+1)
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
		for RawGetInt(l, 1, pos); pos < size; pos++ {
			RawGetInt(l, 1, pos+1)
			RawSetInt(l, 1, pos) // t[pos] = t[pos+1]
		}
		PushNil(l)
		RawSetInt(l, 1, pos) // t[pos] = nil
		return 1
	}},
	// {"sort", sort},
}

func TableOpen(l *State) int {
	NewLibrary(l, tableLibrary)
	return 1
}
