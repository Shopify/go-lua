package lua

import (
	"strings"
)

var stringLibrary = []RegistryFunction{
	// {"byte", ...},
	// {"char", ...},
	// {"dump", ...},
	// {"find", ...},
	// {"format", ...},
	// {"gmatch", ...},
	// {"gsub", ...},
	{"len", func(l *State) int { PushInteger(l, len(CheckString(l, 1))); return 1 }},
	{"lower", func(l *State) int { PushString(l, strings.ToLower(CheckString(l, 1))); return 1 }},
	// {"match", ...},
	{"rep", func(l *State) int {
		s, n, sep := CheckString(l, 1), CheckInteger(l, 2), OptString(l, 3, "")
		if n <= 0 {
			PushString(l, "")
		} else if len(s)+len(sep) < len(s) || len(s)+len(sep) >= maxInt/n {
			Errorf(l, "resulting string too large")
		} else {
			result := s
			for ; n > 1; n-- {
				result += sep + s
			}
			PushString(l, result)
		}
		return 1
	}},
	{"reverse", func(l *State) int {
		r := []rune(CheckString(l, 1))
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		PushString(l, string(r))
		return 1
	}},
	// {"sub", ...},
	{"upper", func(l *State) int { PushString(l, strings.ToUpper(CheckString(l, 1))); return 1 }},
}

func StringOpen(l *State) int {
	NewLibrary(l, stringLibrary)
	CreateTable(l, 0, 1)
	PushString(l, "")
	PushValue(l, -2)
	SetMetaTable(l, -2)
	Pop(l, 1)
	PushValue(l, -2)
	SetField(l, -2, "__index")
	Pop(l, 1)
	return 1
}
