package lua

import "fmt"

func (l *State) resetHookCount() { l.hookCount = l.baseHookCount }

func (l *State) runtimeError(message string) {
	l.push(message)
	if l.callInfo.isLua() {
		//...
	}
	l.errorMessage()
}

func (l *State) typeError(v value, operation string) {
	typeName := TypeName(l, l.valueToType(v))
	if l.callInfo.isLua() {
		var kind, name string
		isUpValue := func() bool {
			c := l.stack[l.callInfo.function()].(*luaClosure)
			for i, uv := range c.upValues {
				if uv.value() == v {
					kind, name = "upvalue", c.prototype.upValues[i].name
					return true
				}
			}
			return false
		}
		isInStack := func() bool {
			for _, e := range l.callInfo.(*luaCallInfo).frame {
				if e == v {
					return true
				}
			}
			return false
		}
		if !isUpValue() && isInStack() {
			// TODO
		}
		if true { // TODO
			l.runtimeError(fmt.Sprintf("attempt to %s %s '%s' (a %s value)", operation, kind, name, typeName))
		}
	}
	l.runtimeError(fmt.Sprintf("attempt to %s a %s value", operation, typeName))
}

func (l *State) orderError(left, right value) {
	leftType, rightType := TypeName(l, l.valueToType(left)), TypeName(l, l.valueToType(right))
	if leftType == rightType {
		l.runtimeError(fmt.Sprintf("attempt to compare two '%s' values", leftType))
	}
	l.runtimeError(fmt.Sprintf("attempt to compare '%s' with '%s'", leftType, rightType))
}

func (l *State) arithError(v1, v2 value) {
	if _, ok := l.toNumber(v1); !ok {
		v2 = v1
	}
	l.typeError(v2, "perform arithmetic on")
}

func (l *State) concatError(v1, v2 value) {
	_, isString := v1.(string)
	_, isNumber := v1.(float64)
	if isString || isNumber {
		v1 = v2
	}
	_, isString = v1.(string)
	_, isNumber = v1.(float64)
	l.assert(!isString && !isNumber)
	l.typeError(v1, "concatenate")
}

func (l *State) assert(cond bool) {
	if !cond {
		l.runtimeError("assertion failure")
	}
}

func (l *State) errorMessage() {
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

func SetHooker(l *State, f Hook, mask byte, count int) {
	if f == nil || mask == 0 {
		f, mask = nil, 0
	}
	if ci, ok := l.callInfo.(*luaCallInfo); ok {
		l.oldPC = ci.savedPC
	}
	l.hooker, l.baseHookCount = f, count
	l.resetHookCount()
	l.hookMask = mask
}

func Hooker(l *State) Hook     { return l.hooker }
func HookerMask(l *State) byte { return l.hookMask }
func HookerCount(l *State) int { return l.hookCount }

func Stack(l *State, level int, activationRecord *Debug) (ok bool) {
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

func Info(l *State, what string, activationRecord *Debug) bool {
	var f closure
	var fun value
	var callInfo callInfo
	if what[0] == '>' {
		fun = l.stack[l.top-1]
		switch fun := fun.(type) {
		case *goClosure:
			f = fun
		case *luaClosure:
			f = fun
		case Function:
		default:
			apiCheck(false, "function expected")
		}
		what = what[1:] // skip the '>'
		l.top--         // pop function
	} else {
		callInfo = activationRecord.callInfo
		fun = l.stack[callInfo.function()]
		switch fun := fun.(type) {
		case *goClosure:
			f = fun
		case *luaClosure:
			f = fun
		case Function:
		default:
			l.assert(false)
		}
	}
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
			if callInfo != nil && !callInfo.isCallStatus(callStatusTail) && callInfo.previous().isLua() {
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
