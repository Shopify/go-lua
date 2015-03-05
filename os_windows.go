package lua

func clock(l *State) int {
	Errorf(l, "os.clock not yet supported on Windows")
	panic("unreachable")
}
