package lua

var tableLibrary = []RegistryFunction{
	// {"concat", tconcat},
	// {"insert", tinsert},
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
	// {"remove", tremove},
	// {"sort", sort},
}

func TableOpen(l *State) int {
	NewLibrary(l, tableLibrary)
	return 1
}
