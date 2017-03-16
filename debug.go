package lua

import (
	"fmt"
	"strings"
)

// A Frame is a token representing an activation record. It is returned by
// Stack and passed to Info.
type Frame *callInfo

func (l *State) resetHookCount() { l.hookCount = l.baseHookCount }
func (l *State) prototype(ci *callInfo) *prototype {
	return l.stack[ci.function].(*luaClosure).prototype
}
func (l *State) currentLine(ci *callInfo) int {
	return int(l.prototype(ci).lineInfo[ci.savedPC])
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
	if ci := l.callInfo; ci.isLua() {
		line, source := l.currentLine(ci), l.prototype(ci).source
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
	typeName := l.valueToType(v).String()
	if ci := l.callInfo; ci.isLua() {
		c := l.stack[ci.function].(*luaClosure)
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
			for i, e := range ci.frame {
				if e == v {
					frameIndex = i
					return true
				}
			}
			return false
		}
		if !isUpValue() && isInStack() {
			name, kind = c.prototype.objectName(frameIndex, ci.savedPC)
		}
		if kind != "" {
			l.runtimeError(fmt.Sprintf("attempt to %s %s '%s' (a %s value)", operation, kind, name, typeName))
		}
	}
	l.runtimeError(fmt.Sprintf("attempt to %s a %s value", operation, typeName))
}

func (l *State) orderError(left, right value) {
	leftType, rightType := l.valueToType(left).String(), l.valueToType(right).String()
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
	l.throw(RuntimeError(CheckString(l, -1)))
}

// SetDebugHook sets the debugging hook function.
//
// f is the hook function. mask specifies on which events the hook will be
// called: it is formed by a bitwise or of the constants MaskCall, MaskReturn,
// MaskLine, and MaskCount. The count argument is only meaningful when the
// mask includes MaskCount. For each event, the hook is called as explained
// below:
//
// Call hook is called when the interpreter calls a function. The hook is
// called just after Lua enters the new function, before the function gets
// its arguments.
//
// Return hook is called when the interpreter returns from a function. The
// hook is called just before Lua leaves the function. There is no standard
// way to access the values to be returned by the function.
//
// Line hook is called when the interpreter is about to start the execution
// of a new line of code, or when it jumps back in the code (even to the same
// line). (This event only happens while Lua is executing a Lua function.)
//
// Count hook is called after the interpreter executes every count
// instructions. (This event only happens while Lua is executing a Lua
// function.)
//
// A hook is disabled by setting mask to zero.
func SetDebugHook(l *State, f Hook, mask byte, count int) {
	if f == nil || mask == 0 {
		f, mask = nil, 0
	}
	if ci := l.callInfo; ci.isLua() {
		l.oldPC = ci.savedPC
	}
	l.hooker, l.baseHookCount = f, count
	l.resetHookCount()
	l.hookMask = mask
	l.internalHook = false
}

// DebugHook returns the current hook function.
func DebugHook(l *State) Hook { return l.hooker }

// DebugHookMask returns the current hook mask.
func DebugHookMask(l *State) byte { return l.hookMask }

// DebugHookCount returns the current hook count.
func DebugHookCount(l *State) int { return l.hookCount }

// Stack gets information about the interpreter runtime stack.
//
// It returns a Frame identifying the activation record of the
// function executing at a given level. Level 0 is the current running
// function, whereas level n+1 is the function that has called level n (except
// for tail calls, which do not count on the stack). When there are no errors,
// Stack returns true; when called with a level greater than the stack depth,
// it returns false.
func Stack(l *State, level int) (f Frame, ok bool) {
	if level < 0 {
		return // invalid (negative) level
	}
	callInfo := l.callInfo
	for ; level > 0 && callInfo != &l.baseCallInfo; level, callInfo = level-1, callInfo.previous {
	}
	if level == 0 && callInfo != &l.baseCallInfo { // level found?
		f, ok = callInfo, true
	}
	return
}

func functionInfo(p Debug, f closure) (d Debug) {
	d = p
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
	return
}

func (l *State) functionName(ci *callInfo) (name, kind string) {
	var tm tm
	p := l.prototype(ci)
	pc := ci.savedPC
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

// Info gets information about a specific function or function invocation.
//
// To get information about a function invocation, the parameter where must
// be a valid activation record that was filled by a previous call to Stack
// or given as an argument to a hook (see Hook).
//
// To get information about a function you push it onto the stack and start
// the what string with the character '>'. (In that case, Info pops the
// function from the top of the stack.) For instance, to know in which line
// a function f was defined, you can write the following code:
//   l.Global("f") // Get global 'f'.
//   d, _ := lua.Info(l, ">S", nil)
//   fmt.Printf("%d\n", d.LineDefined)
//
// Each character in the string what selects some fields of the Debug struct
// to be filled or a value to be pushed on the stack:
// 	 'n': fills in the field Name and NameKind
// 	 'S': fills in the fields Source, ShortSource, LineDefined, LastLineDefined, and What
// 	 'l': fills in the field CurrentLine
// 	 't': fills in the field IsTailCall
// 	 'u': fills in the fields UpValueCount, ParameterCount, and IsVarArg
// 	 'f': pushes onto the stack the function that is running at the given level
// 	 'L': pushes onto the stack a table whose indices are the numbers of the lines that are valid on the function
// (A valid line is a line with some associated code, that is, a line where you
// can put a break point. Non-valid lines include empty lines and comments.)
//
// This function returns false on error (for instance, an invalid option in what).
func Info(l *State, what string, where Frame) (d Debug, ok bool) {
	var f closure
	var fun value
	if what[0] == '>' {
		where = nil
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
		fun = l.stack[where.function]
		switch fun := fun.(type) {
		case closure:
			f = fun
		case *goFunction:
		default:
			l.assert(false)
		}
	}
	ok, hasL, hasF := true, false, false
	d.callInfo = where
	ci := d.callInfo
	for _, r := range what {
		switch r {
		case 'S':
			d = functionInfo(d, f)
		case 'l':
			d.CurrentLine = -1
			if where != nil && ci.isLua() {
				d.CurrentLine = l.currentLine(where)
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
			d.IsTailCall = where != nil && ci.isCallStatus(callStatusTail)
		case 'n':
			// calling function is a known Lua function?
			if where != nil && !ci.isCallStatus(callStatusTail) && where.previous.isLua() {
				d.Name, d.NameKind = l.functionName(where.previous)
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
	return d, ok
}

func upValueHelper(f func(*State, int, int) (string, bool), returnValueCount int) Function {
	return func(l *State) int {
		CheckType(l, 1, TypeFunction)
		name, ok := f(l, 1, CheckInteger(l, 2))
		if !ok {
			return 0
		}
		l.PushString(name)
		l.Insert(-returnValueCount)
		return returnValueCount
	}
}

func (l *State) checkUpValue(f, upValueCount int) int {
	n := CheckInteger(l, upValueCount)
	CheckType(l, f, TypeFunction)
	l.PushValue(f)
	debug, _ := Info(l, ">u", nil)
	ArgumentCheck(l, 1 <= n && n <= debug.UpValueCount, upValueCount, "invalue upvalue index")
	return n
}

func threadArg(l *State) (int, *State) {
	if l.IsThread(1) {
		return 1, l.ToThread(1)
	}
	return 0, l
}

func hookTable(l *State) bool { return SubTable(l, RegistryIndex, "_HKEY") }

func internalHook(l *State, d Debug) {
	hookNames := []string{"call", "return", "line", "count", "tail call"}
	hookTable(l)
	l.PushThread()
	l.RawGet(-2)
	if l.IsFunction(-1) {
		l.PushString(hookNames[d.Event])
		if d.CurrentLine >= 0 {
			l.PushInteger(d.CurrentLine)
		} else {
			l.PushNil()
		}
		_, ok := Info(l, "lS", d.callInfo)
		l.assert(ok)
		l.Call(2, 0)
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
		if l.TypeOf(1) != TypeUserData {
			l.PushNil()
		} else {
			l.UserValue(1)
		}
		return 1
	}},
	{"gethook", func(l *State) int {
		_, l1 := threadArg(l)
		hooker, mask := DebugHook(l1), DebugHookMask(l1)
		if hooker != nil && !l.internalHook {
			l.PushString("external hook")
		} else {
			hookTable(l)
			l1.PushThread()
			//			XMove(l1, l, 1)
			panic("XMove not implemented yet")
			l.RawGet(-2)
			l.Remove(-2)
		}
		l.PushString(maskToString(mask))
		l.PushInteger(DebugHookCount(l1))
		return 3
	}},
	// {"getinfo", db_getinfo},
	// {"getlocal", db_getlocal},
	{"getregistry", func(l *State) int { l.PushValue(RegistryIndex); return 1 }},
	{"getmetatable", func(l *State) int {
		CheckAny(l, 1)
		if !l.MetaTable(1) {
			l.PushNil()
		}
		return 1
	}},
	{"getupvalue", upValueHelper(UpValue, 2)},
	{"upvaluejoin", func(l *State) int {
		n1 := l.checkUpValue(1, 2)
		n2 := l.checkUpValue(3, 4)
		ArgumentCheck(l, !l.IsGoFunction(1), 1, "Lua function expected")
		ArgumentCheck(l, !l.IsGoFunction(3), 3, "Lua function expected")
		UpValueJoin(l, 1, n1, 3, n2)
		return 0
	}},
	{"upvalueid", func(l *State) int { l.PushLightUserData(UpValueId(l, 1, l.checkUpValue(1, 2))); return 1 }},
	{"setuservalue", func(l *State) int {
		if l.TypeOf(1) == TypeLightUserData {
			ArgumentError(l, 1, "full userdata expected, got light userdata")
		}
		CheckType(l, 1, TypeUserData)
		if !l.IsNoneOrNil(2) {
			CheckType(l, 2, TypeTable)
		}
		l.SetTop(2)
		l.SetUserValue(1)
		return 1
	}},
	{"sethook", func(l *State) int {
		var hook Hook
		var mask byte
		var count int
		i, l1 := threadArg(l)
		if l.IsNoneOrNil(i + 1) {
			l.SetTop(i + 1)
		} else {
			s := CheckString(l, i+2)
			CheckType(l, i+1, TypeFunction)
			count = OptInteger(l, i+3, 0)
			hook, mask = internalHook, stringToMask(s, count > 0)
		}
		if !hookTable(l) {
			l.PushString("k")
			l.SetField(-2, "__mode")
			l.PushValue(-1)
			l.SetMetaTable(-2)
		}
		l1.PushThread()
		//	 	XMove(l1, l, 1)
		panic("XMove not yet implemented")
		l.PushValue(i + 1)
		l.RawSet(-3)
		SetDebugHook(l1, hook, mask, count)
		l1.internalHook = true
		return 0
	}},
	// {"setlocal", db_setlocal},
	{"setmetatable", func(l *State) int {
		t := l.TypeOf(2)
		ArgumentCheck(l, t == TypeNil || t == TypeTable, 2, "nil or table expected")
		l.SetTop(2)
		l.SetMetaTable(1)
		return 1
	}},
	{"setupvalue", upValueHelper(SetUpValue, 1)},
	{"traceback", func(l *State) int {
		i, l1 := threadArg(l)
		if s, ok := l.ToString(i + 1); !ok && !l.IsNoneOrNil(i+1) {
			l.PushValue(i + 1)
		} else if l == l1 {
			Traceback(l, l, s, OptInteger(l, i+2, 1))
		} else {
			Traceback(l, l1, s, OptInteger(l, i+2, 0))
		}
		return 1
	}},
}

// DebugOpen opens the debug library. Usually passed to Require.
func DebugOpen(l *State) int {
	NewLibrary(l, debugLibrary)
	return 1
}
