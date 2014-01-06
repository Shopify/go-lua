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

func (l *state) errorMessage() {
	if l.errorFunction != 0 { // is there an error handling function?
		if errorFunction, ok := l.stack[l.errorFunction].(*luaClosure); ok {
			l.stack[l.top] = l.stack[l.top-1] // move argument
			l.stack[l.top-1] = errorFunction  // push function
			l.top++
			l.call(l.top-2, 1, false)
		} else {
			l.throw(ErrorError)
		}
	}
	l.throw(RuntimeError)
}

func (l *state) Stack(level int, activationRecord *Debug) (ok bool) {
	if level < 0 {
		return // invalid (negative) level
	}
	callInfo := l.callInfo
	for ; level > 0 && callInfo != &l.baseCallInfo; level, callInfo = level-1, callInfo.previous() {
	}
	if level == 0 && callInfo != &l.baseCallInfo { // level found?
		activationRecord.callInfo, ok = callInfo, true
	}
	return
}

func (l *state) Info(what string, activationRecord *Debug) bool {
	var f closure
	var callInfo callInfo
	if what[0] == '>' {
		_, ok := l.stack[l.top-1].(closure)
		apiCheck(ok, "function expected")
		what = what[1:] // skip the '>'
		l.top--         // pop function
	} else {
		callInfo = activationRecord.callInfo
		_, ok := l.stack[callInfo.function()].(closure)
		l.assert(ok)
	}
	// TODO cl = ttisclosure(f) ? clvalue(f) : NULL;
	ok, hasL, hasF := true, false, false
	for _, r := range what {
		switch r {
		case 'S':
			// TODO functionInfo(activationRecord, f)
		case 'l':
			activationRecord.CurrentLine = -1
			if callInfo != nil && callInfo.isLua() {
				// TODO activationRecord.CurrentLine = currentLine(callInfo)
			}
		case 'u':
			if f == nil {
				activationRecord.UpValueCount = 0
			} else {
				activationRecord.UpValueCount = f.upValueCount()
			}
			if lf, ok := f.(*luaClosure); !ok {
				activationRecord.IsVarArg = true
				activationRecord.ParameterCount = 0
			} else {
				activationRecord.IsVarArg = lf.prototype.isVarArg
				activationRecord.ParameterCount = lf.prototype.parameterCount
			}
		case 't':
			activationRecord.IsTailCall = callInfo != nil && callInfo.callStatus()&callStatusTail != 0
		case 'n':
			// calling function is a known Lua function?
			if callInfo != nil && callInfo.callStatus()&callStatusTail == 0 && callInfo.previous().isLua() {
				// TODO activationRecord.Name, activationRecord.NameKind = functionName(l, callInfo.previous())
			} else {
				activationRecord.NameKind = ""
			}
			if activationRecord.NameKind == "" {
				activationRecord.NameKind = "" // not found
				activationRecord.Name = ""
			}
		case 'L':
			hasL = true
		case 'f':
			hasF = true
		default:
			ok = false
		}
	}
	if hasF {
		l.apiPush(f)
	}
	if hasL {
		// TODO collectValidLines(l, cl)
	}
	return ok
}
