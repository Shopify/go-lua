package lua

import (
	"fmt"
	"strings"
)

func (l *State) resetHookCount() { l.hookCount = l.baseHookCount }
func (l *State) prototype(ci callInfo) *prototype {
	return l.stack[ci.function()].(*luaClosure).prototype
}
func (l *State) currentLine(ci callInfo) int {
	return int(l.prototype(ci).lineInfo[ci.(*luaCallInfo).savedPC])
}

func chunkID(source string) string {
	switch source[0] {
	case '=': // "literal" source
		if len(source) <= idSize {
			return source[1:]
		}
		return source[1:idSize]
	case '@': // file name
		if len(source) <= idSize {
			return source[1:]
		}
		return "..." + source[1:idSize-3]
	}
	source = strings.Split(source, "\n")[0]
	if l := len("[string \"...\"]"); len(source) > idSize-l {
		return "[string \"" + source + "...\"]"
	}
	return "[string \"" + source + "\"]"
}

func (l *State) runtimeError(message string) {
	l.push(message)
	if l.callInfo.isLua() {
		line, source := l.currentLine(l.callInfo), l.prototype(l.callInfo).source
		if source == "" {
			source = "?"
		} else {
			source = chunkID(source)
		}
		l.push(fmt.Sprintf("%s:%d: %s", source, line, message))
	}
	l.errorMessage()
}

func (l *State) typeError(v value, operation string) {
	typeName := TypeName(l, l.valueToType(v))
	if l.callInfo.isLua() {
		c := l.stack[l.callInfo.function()].(*luaClosure)
		var kind, name string
		isUpValue := func() bool {
			for i, uv := range c.upValues {
				if uv.value() == v {
					kind, name = "upvalue", c.prototype.upValueName(i)
					return true
				}
			}
			return false
		}
		frameIndex := 0
		isInStack := func() bool {
			for i, e := range l.callInfo.(*luaCallInfo).frame {
				if e == v {
					frameIndex = i
					return true
				}
			}
			return false
		}
		if !isUpValue() && isInStack() {
			kind, name = c.prototype.objectName(frameIndex, l.callInfo.(*luaCallInfo).savedPC)
		}
		if kind != "" {
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
		errorFunction := l.stack[l.errorFunction]
		switch errorFunction.(type) {
		case closure:
		case *goFunction:
		default:
			l.throw(ErrorError)
		}
		l.stack[l.top] = l.stack[l.top-1] // move argument
		l.stack[l.top-1] = errorFunction  // push function
		l.top++
		l.call(l.top-2, 1, false)
	}
	l.throw(fmt.Errorf("%v: %s", RuntimeError, CheckString(l, -1)))
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
	l.internalHook = false
}

func Hooker(l *State) Hook     { return l.hooker }
func HookerMask(l *State) byte { return l.hookMask }
func HookerCount(l *State) int { return l.hookCount }

func Stack(l *State, level int, d *Debug) (ok bool) {
	if level < 0 {
		return // invalid (negative) level
	}
	callInfo := l.callInfo
	for ; level > 0 && callInfo != &l.baseCallInfo; level, callInfo = level-1, callInfo.previous() {
	}
	if level == 0 && callInfo != &l.baseCallInfo { // level found?
		d.callInfo, ok = callInfo, true
	}
	return
}

func functionInfo(d *Debug, f closure) {
	if l, ok := f.(*luaClosure); !ok {
		d.Source = "=[Go]"
		d.LineDefined, d.LastLineDefined = -1, -1
		d.What = "Go"
	} else {
		p := l.prototype
		d.Source = p.source
		if d.Source == "" {
			d.Source = "=?"
		}
		d.LineDefined, d.LastLineDefined = p.lineDefined, p.lastLineDefined
		d.What = "Lua"
		if d.LineDefined == 0 {
			d.What = "main"
		}
	}
	d.ShortSource = chunkID(d.Source)
}

func (l *State) functionName(ci callInfo) (name, kind string) {
	var tm tm
	p := l.prototype(ci)
	pc := ci.(*luaCallInfo).savedPC
	switch i := p.code[pc]; i.opCode() {
	case opCall, opTailCall:
		return p.objectName(i.a(), pc)
	case opTForCall:
		return "for iterator", "for iterator"
	case opSelf, opGetTableUp, opGetTable:
		tm = tmIndex
	case opSetTableUp, opSetTable:
		tm = tmNewIndex
	case opEqual:
		tm = tmEq
	case opAdd:
		tm = tmAdd
	case opSub:
		tm = tmSub
	case opMul:
		tm = tmMul
	case opDiv:
		tm = tmDiv
	case opMod:
		tm = tmMod
	case opPow:
		tm = tmPow
	case opUnaryMinus:
		tm = tmUnaryMinus
	case opLength:
		tm = tmLen
	case opLessThan:
		tm = tmLT
	case opLessOrEqual:
		tm = tmLE
	case opConcat:
		tm = tmConcat
	default:
		return
	}
	return eventNames[tm], "metamethod"
}

func (l *State) collectValidLines(f closure) {
	if lc, ok := f.(*luaClosure); !ok {
		l.apiPush(nil)
	} else {
		t := newTable()
		l.apiPush(t)
		for _, i := range lc.prototype.lineInfo {
			t.putAtInt(int(i), true)
		}
	}
}

func Info(l *State, what string, d *Debug) bool {
	var f closure
	var fun value
	var callInfo callInfo
	if what[0] == '>' {
		fun = l.stack[l.top-1]
		switch fun := fun.(type) {
		case closure:
			f = fun
		case *goFunction:
		default:
			panic("function expected")
		}
		what = what[1:] // skip the '>'
		l.top--         // pop function
	} else {
		callInfo = d.callInfo
		fun = l.stack[callInfo.function()]
		switch fun := fun.(type) {
		case closure:
			f = fun
		case *goFunction:
		default:
			l.assert(false)
		}
	}
	ok, hasL, hasF := true, false, false
	for _, r := range what {
		switch r {
		case 'S':
			functionInfo(d, f)
		case 'l':
			d.CurrentLine = -1
			if callInfo != nil && callInfo.isLua() {
				d.CurrentLine = l.currentLine(callInfo)
			}
		case 'u':
			if f == nil {
				d.UpValueCount = 0
			} else {
				d.UpValueCount = f.upValueCount()
			}
			if lf, ok := f.(*luaClosure); !ok {
				d.IsVarArg = true
				d.ParameterCount = 0
			} else {
				d.IsVarArg = lf.prototype.isVarArg
				d.ParameterCount = lf.prototype.parameterCount
			}
		case 't':
			d.IsTailCall = callInfo != nil && callInfo.callStatus()&callStatusTail != 0
		case 'n':
			// calling function is a known Lua function?
			if callInfo != nil && !callInfo.isCallStatus(callStatusTail) && callInfo.previous().isLua() {
				d.Name, d.NameKind = l.functionName(callInfo.previous())
			} else {
				d.NameKind = ""
			}
			if d.NameKind == "" {
				d.NameKind = "" // not found
				d.Name = ""
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
		l.collectValidLines(f)
	}
	return ok
}

func upValueHelper(f func(*State, int, int) (string, bool), returnValueCount int) Function {
	return func(l *State) int {
		CheckType(l, 1, TypeFunction)
		if name, ok := f(l, 1, CheckInteger(l, 2)); !ok {
			return 0
		} else {
			PushString(l, name)
		}
		Insert(l, -returnValueCount)
		return returnValueCount
	}
}

func (l *State) checkUpValue(f, upValueCount int) int {
	n := CheckInteger(l, upValueCount)
	CheckType(l, f, TypeFunction)
	PushValue(l, f)
	var debug Debug
	Info(l, ">u", &debug)
	ArgumentCheck(l, 1 <= n && n <= debug.UpValueCount, upValueCount, "invalue upvalue index")
	return n
}

func threadArg(l *State) (int, *State) {
	if IsThread(l, 1) {
		return 1, ToThread(l, 1)
	}
	return 0, l
}

func hookTable(l *State) bool { return SubTable(l, RegistryIndex, "_HKEY") }

func internalHooker(l *State, d *Debug) {
	hookNames := []string{"call", "return", "line", "count", "tail call"}
	hookTable(l)
	PushThread(l)
	RawGet(l, -2)
	if IsFunction(l, -1) {
		PushString(l, hookNames[d.Event])
		if d.CurrentLine >= 0 {
			PushInteger(l, d.CurrentLine)
		} else {
			PushNil(l)
		}
		l.assert(Info(l, "lS", d))
		Call(l, 2, 0)
	}
}

func maskToString(mask byte) (s string) {
	if mask&MaskCall != 0 {
		s += "c"
	}
	if mask&MaskReturn != 0 {
		s += "r"
	}
	if mask&MaskLine != 0 {
		s += "l"
	}
	return
}

func stringToMask(s string, maskCount bool) (mask byte) {
	for r, b := range map[rune]byte{'c': MaskCall, 'r': MaskReturn, 'l': MaskLine} {
		if strings.ContainsRune(s, r) {
			mask |= b
		}
	}
	if maskCount {
		mask |= MaskCount
	}
	return
}

var debugLibrary = []RegistryFunction{
	// {"debug", db_debug},
	{"getuservalue", func(l *State) int {
		if TypeOf(l, 1) != TypeUserData {
			PushNil(l)
		} else {
			UserValue(l, 1)
		}
		return 1
	}},
	{"gethook", func(l *State) int {
		_, l1 := threadArg(l)
		hooker, mask := Hooker(l1), HookerMask(l1)
		if hooker != nil && !l.internalHook {
			PushString(l, "external hook")
		} else {
			hookTable(l)
			PushThread(l1)
			//			XMove(l1, l, 1)
			panic("XMove not implemented yet")
			RawGet(l, -2)
			Remove(l, -2)
		}
		PushString(l, maskToString(mask))
		PushInteger(l, HookerCount(l1))
		return 3
	}},
	// {"getinfo", db_getinfo},
	// {"getlocal", db_getlocal},
	{"getregistry", func(l *State) int { PushValue(l, RegistryIndex); return 1 }},
	{"getmetatable", func(l *State) int {
		CheckAny(l, 1)
		if !MetaTable(l, 1) {
			PushNil(l)
		}
		return 1
	}},
	{"getupvalue", upValueHelper(UpValue, 2)},
	{"upvaluejoin", func(l *State) int {
		n1 := l.checkUpValue(1, 2)
		n2 := l.checkUpValue(3, 4)
		ArgumentCheck(l, !IsGoFunction(l, 1), 1, "Lua function expected")
		ArgumentCheck(l, !IsGoFunction(l, 3), 3, "Lua function expected")
		UpValueJoin(l, 1, n1, 3, n2)
		return 0
	}},
	{"upvalueid", func(l *State) int { PushLightUserData(l, UpValueId(l, 1, l.checkUpValue(1, 2))); return 1 }},
	{"setuservalue", func(l *State) int {
		if TypeOf(l, 1) == TypeLightUserData {
			ArgumentError(l, 1, "full userdata expected, got light userdata")
		}
		CheckType(l, 1, TypeUserData)
		if !IsNoneOrNil(l, 2) {
			CheckType(l, 2, TypeTable)
		}
		SetTop(l, 2)
		SetUserValue(l, 1)
		return 1
	}},
	{"sethook", func(l *State) int {
		var hooker Hook
		var mask byte
		var count int
		i, l1 := threadArg(l)
		if IsNoneOrNil(l, i+1) {
			SetTop(l, i+1)
		} else {
			s := CheckString(l, i+2)
			CheckType(l, i+1, TypeFunction)
			count = OptInteger(l, i+3, 0)
			hooker, mask = internalHooker, stringToMask(s, count > 0)
		}
		if !hookTable(l) {
			PushString(l, "k")
			SetField(l, -2, "__mode")
			PushValue(l, -1)
			SetMetaTable(l, -2)
		}
		PushThread(l1)
		//	 	XMove(l1, l, 1)
		panic("XMove not yet implemented")
		PushValue(l, i+1)
		RawSet(l, -3)
		SetHooker(l1, hooker, mask, count)
		l1.internalHook = true
		return 0
	}},
	// {"setlocal", db_setlocal},
	{"setmetatable", func(l *State) int {
		t := TypeOf(l, 2)
		ArgumentCheck(l, t == TypeNil || t == TypeTable, 2, "nil or table expected")
		SetTop(l, 2)
		SetMetaTable(l, 1)
		return 1
	}},
	{"setupvalue", upValueHelper(SetUpValue, 1)},
	{"traceback", func(l *State) int {
		i, l1 := threadArg(l)
		if s, ok := ToString(l, i+1); !ok && !IsNoneOrNil(l, i+1) {
			PushValue(l, i+1)
		} else if l == l1 {
			Traceback(l, l, s, OptInteger(l, i+2, 1))
		} else {
			Traceback(l, l1, s, OptInteger(l, i+2, 0))
		}
		return 1
	}},
}

func DebugOpen(l *State) int {
	NewLibrary(l, debugLibrary)
	return 1
}
