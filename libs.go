package lua

func OpenLibraries(l *State, preloaded ...RegistryFunction) {
	libs := []RegistryFunction{
		{"_G", BaseOpen},
		{"package", PackageOpen},
		{"table", TableOpen},
		{"string", StringOpen},
		{"bit32", Bit32Open},
		{"math", MathOpen},
	}
	for _, lib := range libs {
		Require(l, lib.Name, lib.Function, true)
		Pop(l, 1)
	}
	SubTable(l, RegistryIndex, "_PRELOAD")
	for _, lib := range preloaded {
		PushGoFunction(l, lib.Function)
		SetField(l, -2, lib.Name)
	}
	Pop(l, 1)
}
