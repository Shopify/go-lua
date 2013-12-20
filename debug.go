package lua

func (l *state) runtimeError(message string) { // TODO
	panic("runtimeError")
}

func (l *state) typeError(v value, message string) { // TODO
	panic("typeError")
}

func (l *state) orderError(left, right value) { // TODO
	panic("orderError")
}

func (l *state) arithError(v1, v2 value) { // TODO
	panic("arithError")
}

func (l *state) concatError(v1, v2 value) { // TODO
	panic("concatError")
}

func (l *state) assert(cond bool) {
	if !cond {
		l.runtimeError("assertion failure")
	}
}
