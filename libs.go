package lua

func OpenLibraries(l *State) {
	libs := []RegistryFunction{
		{"_G", BaseOpen},
		{"package", PackageOpen},
		{"math", MathOpen},
	}
	for _, lib := range libs {
		Require(l, lib.Name, lib.Function, true)
		Pop(l, 1)
	}
	// TODO support preloaded libraries
}
