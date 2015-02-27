package lua

// OpenLibraries opens all standard libraries. Alternatively, the host program
// can open them individually by using Require to call BaseOpen (for the basic
// library), PackageOpen (for the package library), CoroutineOpen (for the
// coroutine library), StringOpen (for the string library), TableOpen (for the
// table library), MathOpen (for the mathematical library), Bit32Open (for the
// bit library), IOOpen (for the I/O library), OSOpen (for the Operating System
// library), and DebugOpen (for the debug library).
//
// The standard Lua libraries provide useful functions that are implemented
// directly through the Go API. Some of these functions provide essential
// services to the language (e.g. Type and MetaTable); others provide access
// to "outside" services (e.g. I/O); and others could be implemented in Lua
// itself, but are quite useful or have critical performance requirements that
// deserve an implementation in Go (e.g. table.sort).
//
// All libraries are implemented through the official Go API. Currently, Lua
// has the following standard libraries:
//  basic library
//  package library
//  string manipulation
//  table manipulation
//  mathematical functions (sin, log, etc.);
//  bitwise operations
//  input and output
//  operating system facilities
//  debug facilities
// Except for the basic and the package libraries, each library provides all
// its functions as fields of a global table or as methods of its objects.
func OpenLibraries(l *State, preloaded ...RegistryFunction) {
	libs := []RegistryFunction{
		{"_G", BaseOpen},
		{"package", PackageOpen},
		// {"coroutine", CoroutineOpen},
		{"table", TableOpen},
		{"io", IOOpen},
		{"os", OSOpen},
		{"string", StringOpen},
		{"bit32", Bit32Open},
		{"math", MathOpen},
		{"debug", DebugOpen},
	}
	for _, lib := range libs {
		Require(l, lib.Name, lib.Function, true)
		l.Pop(1)
	}
	SubTable(l, RegistryIndex, "_PRELOAD")
	for _, lib := range preloaded {
		l.PushGoFunction(lib.Function)
		l.SetField(-2, lib.Name)
	}
	l.Pop(1)
}
