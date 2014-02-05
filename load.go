package lua

func findLoader(l *State, name string) {
	// TODO accumulate errors?
	if Field(l, UpValueIndex(1), "searchers"); !IsTable(l, 3) {
		Errorf(l, "'package.searchers' must be a table")
	}
	for i := 1; ; i++ {
		if RawGetInt(l, 3, i); IsNil(l, -1) {
			Pop(l, 1)
			// push error message
			Errorf(l, "module '%s' not found: %s", name, "")
		}
		PushString(l, name)
		if Call(l, 1, 2); IsFunction(l, -2) {
			return
		} else if IsString(l, -2) {
			Pop(l, 1)
			// add to error message
		} else {
			Pop(l, 2)
		}
	}
}

func PackageOpen(l *State) int {
	// NewLibrary(l, packageLibrary)
	CreateTable(l, 0, 0)
	SubTable(l, RegistryIndex, "_LOADED")
	SetField(l, -2, "loaded")
	SubTable(l, RegistryIndex, "_PRELOAD")
	SetField(l, -2, "preload")
	PushGlobalTable(l)
	PushValue(l, -2)
	SetFunctions(l, []RegistryFunction{{"require", func(l *State) int {
		name := CheckString(l, 1)
		SetTop(l, 1)
		Field(l, RegistryIndex, "_LOADED")
		Field(l, 2, name)
		if ToBoolean(l, -1) {
			return 1
		}
		Pop(l, 1)
		findLoader(l, name)
		PushString(l, name)
		Insert(l, -2)
		Call(l, 2, 1)
		if !IsNil(l, -1) {
			SetField(l, 2, name)
		}
		Field(l, 2, name)
		if IsNil(l, -1) {
			PushBoolean(l, true)
			PushValue(l, -1)
			SetField(l, 2, name)
		}
		return 1
	}}}, 1)
	Pop(l, 1)
	return 1
}
