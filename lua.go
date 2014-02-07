package lua

import (
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	MultipleReturns = -1 // option for multiple returns in 'PCall' and 'Call'
)

const (
	HookCall, MaskCall = iota, 1 << iota
	HookReturn, MaskReturn
	HookLine, MaskLine
	HookCount, MaskCount
	HookTailCall, MaskTailCall
)

type Status byte

const (
	Ok Status = iota
	Yield
	RuntimeError
	SyntaxError
	MemoryError
	GCError
	ErrorError
	FileError
)

type Type int

const (
	TypeNil Type = iota
	TypeBoolean
	TypeLightUserData
	TypeNumber
	TypeString
	TypeTable
	TypeFunction
	TypeUserData
	TypeThread

	TypeCount
	TypeNone = TypeNil - 1
)

const (
	OpAdd = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPow
	OpUnaryMinus
	OpEq = iota
	OpLT
	OpLE
)

const (
	RegistryIndexMainThread = iota
	RegistryIndexGlobals
)

const (
	Signature     = "\033Lua" // mark for precompiled code ('<esc>Lua')
	VersionMajor  = 5
	VersionMinor  = 2
	VersionNumber = 502
	MinStack      = 20 // minimum Lua stack available to a Go function
	VersionString = "Lua " + string('0'+VersionMajor) + "." + string('0'+VersionMinor)
	RegistryIndex = firstPseudoIndex
)

type RegistryFunction struct {
	Name     string
	Function Function
}

type Debug struct {
	Event                                     int
	Name                                      string
	NameKind                                  string // "global", "local", "field", "method"
	What                                      string // "Lua", "Go", "main", "tail"
	Source                                    string
	CurrentLine, LineDefined, LastLineDefined int
	UpValueCount, ParameterCount              int
	IsVarArg, IsTailCall                      bool
	callInfo                                  callInfo // active function
}

type Hook func(state *State, activationRecord *Debug)
type Function func(state *State) int

// type State interface {
// basic stack manipulation
// TODO XMove(from, to State, n int)

// Access functions (stack -> Go)
// Comparison and arithmetic functions
// Push functions (Go -> stack)

// Get functions (Lua -> stack)
// TODO NewUserData(?) interface{}

// Set functions (stack -> Lua)
// SetTable(index int)
// SetField(index int, name string)
// RawSetInt(index, n int)
// RawSetValue(index int, p interface{})
// SetUserValue(index int)

// Miscellaneous functions
// TODO AllocateFunction() (f Alloc, userData interface{})
// TODO SetAllocateFunction(f Alloc, userData interface{})

// Useful functions

// Debug API
// Local(activationRecord *Debug, index int) string
// SetLocal(activationRecord *Debug, index int) string
// UpValue(function, index int) string
// SetUpValue(function, index int) string
// UpValueId(function, index int) interface{}
// UpValueJoin(function1, index1, function2, index2 int)
// Hook() Hook
// HookMask() int
// HookCount() int
// }

type pc int
type callStatus byte

const (
	callStatusLua                callStatus = 1 << iota // call is running a Lua function
	callStatusHooked                                    // call is running a debug hook
	callStatusReentry                                   // call is running on same invocation of execute of previous call
	callStatusYielded                                   // call reentered after suspension
	callStatusYieldableProtected                        // call is a yieldable protected call
	callStatusError                                     // call has an error status (pcall)
	callStatusTail                                      // call was tail called
	callStatusHookYielded                               // last hook called yielded
)

// per thread state
type State struct {
	status                Status
	top                   int // first free slot in the stack
	global                *globalState
	callInfo              callInfo // call info for current function
	oldPC                 pc       // last pC traced
	stackLast             int      // last free slot in the stack
	stack                 []value
	nonYieldableCallCount int
	nestedGoCallCount     int
	hookMask              byte
	allowHook             bool
	baseHookCount         int
	hookCount             int
	hooker                Hook
	upValues              *openUpValue
	errorFunction         int         // current error handling function (stack index)
	baseCallInfo          luaCallInfo // callInfo for first level (go calling lua)
	protectFunction       func()
}

type globalState struct {
	mainThread         *State
	tagMethodNames     [tmCount]string
	metaTables         [TypeCount]*table // metatables for basic types
	registry           *table
	panicFunction      Function // to be called in unprotected errors
	version            *float64 // pointer to version number
	memoryErrorMessage string
	// seed uint // randomized seed for hashes
	// upValueHead upValue // head of double-linked list of all open upvalues
}

func (g *globalState) metaTable(o value) *table {
	var t Type
	switch o.(type) {
	case nil:
		t = TypeNil
	case bool:
		t = TypeBoolean
	// TODO TypeLightUserData
	case float64:
		t = TypeNumber
	case string:
		t = TypeString
	case *table:
		t = TypeTable
	case Function:
		t = TypeFunction
	case *userData:
		t = TypeUserData
	case *State:
		t = TypeThread
	default:
		return nil
	}
	return g.metaTables[t]
}

func (l *State) adjustResults(resultCount int) {
	if resultCount == MultipleReturns && l.callInfo.top() < l.top {
		l.callInfo.setTop(l.top)
	}
}

func apiCheck(condition bool, message string) {
	if !condition {
		panic(message)
	}
}

func (l *State) apiIncrementTop() {
	l.top++
	apiCheck(l.top <= l.callInfo.top(), "stack overflow")
}

func (l *State) apiPush(v value) {
	l.push(v)
	apiCheck(l.top <= l.callInfo.top(), "stack overflow")
}

func (l *State) checkElementCount(n int) {
	apiCheck(n < l.top-l.callInfo.function(), "not enough elements in the stack")
}

func (l *State) checkResults(argCount, resultCount int) {
	apiCheck(resultCount == MultipleReturns || l.callInfo.top()-l.top >= resultCount-argCount,
		"results from function overflow current stack size")
}

func Context(l *State) (Status, int) {
	if l.callInfo.isCallStatus(callStatusYielded) {
		callInfo := l.callInfo.(*goCallInfo)
		return callInfo.status, callInfo.context
	}
	return Ok, 0
}

func CallWithContinuation(l *State, argCount, resultCount, context int, continuation Function) {
	apiCheck(continuation == nil || !l.callInfo.isLua(), "cannot use continuations inside hooks")
	l.checkElementCount(argCount + 1)
	apiCheck(l.status == Ok, "cannot do calls on non-normal thread")
	l.checkResults(argCount, resultCount)
	f := l.top - (argCount + 1)
	if continuation != nil && l.nonYieldableCallCount == 0 { // need to prepare continuation?
		callInfo := l.callInfo.(*goCallInfo)
		callInfo.continuation = continuation
		callInfo.context = context
		l.call(f, resultCount, true) // just do the call
	} else { // no continuation or not yieldable
		l.call(f, resultCount, false) // just do the call
	}
	l.adjustResults(resultCount)
}

func ProtectedCall(l *State, argCount, resultCount, errorFunction int) Status {
	return ProtectedCallWithContinuation(l, argCount, resultCount, errorFunction, 0, nil)
}

func ProtectedCallWithContinuation(l *State, argCount, resultCount, errorFunction, context int, continuation Function) Status {
	apiCheck(continuation == nil || !l.callInfo.isLua(), "cannot use continuations inside hooks")
	l.checkElementCount(argCount + 1)
	apiCheck(l.status == Ok, "cannot do calls on non-normal thread")
	l.checkResults(argCount, resultCount)
	if errorFunction != 0 {
		apiCheckStackIndex(errorFunction, l.indexToValue(errorFunction))
		errorFunction = AbsIndex(l, errorFunction)
	}
	f := l.top - (argCount + 1)
	status := Ok
	if continuation == nil || l.nonYieldableCallCount > 0 {
		status = l.protectedCall(func() { l.call(f, resultCount, false) }, f, errorFunction)
	} else {
		c := l.callInfo.(*goCallInfo)
		c.continuation, c.context, c.extra, c.oldAllowHook, c.oldErrorFunction = continuation, context, f, l.allowHook, l.errorFunction
		l.errorFunction = errorFunction
		c.setCallStatus(callStatusYieldableProtected)
		l.call(f, resultCount, true)
		c.clearCallStatus(callStatusYieldableProtected)
		l.errorFunction = c.oldErrorFunction
	}
	l.adjustResults(resultCount)
	return status
}

func Load(l *State, r io.Reader, chunkName, mode string) Status {
	if chunkName == "" {
		chunkName = "?"
	}
	status := protectedParser(l, r, chunkName, mode)
	if status == Ok {
		if f := l.stack[l.top-1].(*luaClosure); f.upValueCount() == 1 {
			f.setUpValue(0, l.global.registry.atInt(RegistryIndexGlobals))
		}
	}
	return status
}

func NewState() *State {
	v := float64(VersionNumber)
	l := &State{allowHook: true, status: Ok, nonYieldableCallCount: 1}
	g := &globalState{mainThread: l, registry: newTable(), version: &v, memoryErrorMessage: "not enough memory"}
	l.global = g
	l.initializeStack()
	g.registry.putAtInt(RegistryIndexMainThread, l)
	g.registry.putAtInt(RegistryIndexGlobals, newTable())
	copy(g.tagMethodNames[:], eventNames)
	return l
}

func apiCheckStackIndex(index int, v value) {
	apiCheck(v != nil && !isPseudoIndex(index), "index not in the stack")
}

func SetField(l *State, index int, key string) {
	l.checkElementCount(1)
	t := l.indexToValue(index)
	l.stack[l.top] = key
	l.top++
	l.setTableAt(t, key, l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

func (l *State) indexToValue(index int) value {
	switch callInfo := l.callInfo; {
	case index > 0:
		// TODO are these checks necessary? Can we just return l.callInfo[index]?
		apiCheck(index <= callInfo.top()-(callInfo.function()+1), "unacceptable index")
		if i := callInfo.function() + index; i < l.top {
			return l.stack[i]
		}
		return nil
	case !isPseudoIndex(index): // negative index
		apiCheck(index != 0 && -index <= l.top-(callInfo.function()+1), "invalid index")
		return l.stack[l.top+index]
	case index == RegistryIndex:
		return l.global.registry
	default: // upvalues
		i := RegistryIndex - index
		apiCheck(i <= maxUpValue+1, "upvalue index too large")
		if _, ok := l.stack[callInfo.function()].(Function); ok {
			return nil // light Go functions have no upvalues
		}
		if closure := l.stack[callInfo.function()].(*goClosure); i <= len(closure.upValues) {
			return closure.upValues[i-1]
		}
		return nil
	}
}

func (l *State) setIndexToValue(index int, v value) {
	switch callInfo := l.callInfo; {
	case index > 0:
		// TODO are these checks necessary? Can we just return l.callInfo[index]?
		apiCheck(index <= callInfo.top()-(callInfo.function()+1), "unacceptable index")
		if i := callInfo.function() + index; i < l.top {
			l.stack[i] = v
		}
		panic("unacceptable index")
	case !isPseudoIndex(index): // negative index
		apiCheck(index != 0 && -index <= l.top-(callInfo.function()+1), "invalid index")
		l.stack[l.top+index] = v
	case index == RegistryIndex:
		l.global.registry = v.(*table)
	default: // upvalues
		i := RegistryIndex - index
		apiCheck(i <= maxUpValue+1, "upvalue index too large")
		if _, ok := l.stack[callInfo.function()].(Function); ok {
			panic("light Go functions have no upvalues")
		}
		if closure := l.stack[callInfo.function()].(*goClosure); i <= len(closure.upValues) {
			closure.upValues[i-1] = v
		}
		panic("upvalue index too large")
	}
}

func AbsIndex(l *State, index int) int {
	if index > 0 || isPseudoIndex(index) {
		return index
	}
	return l.top - l.callInfo.function() + index
}

func SetTop(l *State, index int) {
	f := l.callInfo.function()
	if index >= 0 {
		apiCheck(index <= l.stackLast-(f+1), "new top too large")
		i := l.top
		for l.top = f + 1 + index; i < l.top; i++ {
			l.stack[i] = nil
		}
	} else {
		apiCheck(-(index+1) <= l.top-(f+1), "invalid new top")
		l.top += index + 1 // 'subtract' index (index is negative)
	}
}

func Remove(l *State, index int) {
	apiCheckStackIndex(index, l.indexToValue(index))
	i := AbsIndex(l, index)
	copy(l.stack[i:l.top-1], l.stack[i+1:l.top])
	l.top--
}

func Insert(l *State, index int) {
	apiCheckStackIndex(index, l.indexToValue(index))
	i := AbsIndex(l, index)
	copy(l.stack[i+1:l.top+1], l.stack[i:l.top])
	l.stack[i] = l.stack[l.top]
}

func (l *State) move(dest int, src value) {
	apiCheck(src != nil, "invalid index")
	l.setIndexToValue(dest, src)
}

func Replace(l *State, index int) {
	l.checkElementCount(1)
	l.move(index, l.stack[l.top-1])
	l.top--
}

func CheckStack(l *State, size int) bool {
	callInfo := l.callInfo
	ok := l.stackLast-l.top > size
	if !ok && l.top+extraStack <= maxStack-size {
		ok = l.protect(func() { l.growStack(size) }) == Ok
	}
	if ok && callInfo.top() < l.top+size {
		callInfo.setTop(l.top + size)
	}
	return ok
}

func AtPanic(l *State, panicFunction Function) Function {
	panicFunction, l.global.panicFunction = l.global.panicFunction, panicFunction
	return panicFunction
}

func (l *State) valueToType(v value) Type {
	switch v.(type) {
	case nil:
		return TypeNil
	case bool:
		return TypeBoolean
	// case lightUserData:
	// 	return TypeLightUserData
	case float64:
		return TypeNumber
	case string:
		return TypeString
	case *table:
		return TypeTable
	case Function:
		return TypeFunction
	case *userData:
		return TypeUserData
	case *State:
		return TypeThread
	}
	return TypeNone
}

func TypeOf(l *State, index int) Type {
	return l.valueToType(l.indexToValue(index))
}

func IsGoFunction(l *State, index int) bool {
	if _, ok := l.indexToValue(index).(Function); ok {
		return true
	}
	_, ok := l.indexToValue(index).(*goClosure)
	return ok
}

func IsNumber(l *State, index int) bool {
	_, ok := toNumber(l.indexToValue(index))
	return ok
}

func IsString(l *State, index int) bool {
	if _, ok := l.indexToValue(index).(string); ok {
		return true
	}
	_, ok := l.indexToValue(index).(float64)
	return ok
}

func IsUserData(l *State, index int) bool {
	_, ok := l.indexToValue(index).(*userData)
	return ok
}

func Arith(l *State, op int) {
	if op != OpUnaryMinus {
		l.checkElementCount(2)
	} else {
		l.checkElementCount(1)
		l.push(l.stack[l.top-1])
	}
	o1, o2 := l.stack[l.top-2], l.stack[l.top-1]
	if n1, n2, ok := pairAsNumbers(o1, o2); ok {
		l.stack[l.top-2] = arith(op, n1, n2)
	} else {
		l.stack[l.top-2] = l.arith(o1, o2, tm(op-OpAdd)+tmAdd)
	}
	l.top--
}

func RawEqual(l *State, index1, index2 int) bool {
	if o1, o2 := l.indexToValue(index1), l.indexToValue(index2); o1 != nil && o2 != nil {
		return o1 == o2
	}
	return false
}

func Compare(l *State, index1, index2, op int) bool {
	if o1, o2 := l.indexToValue(index1), l.indexToValue(index2); o1 != nil && o2 != nil {
		switch op {
		case OpEq:
			return l.equalObjects(o1, o2)
		case OpLT:
			return l.lessThan(o1, o2)
		case OpLE:
			return l.lessOrEqual(o1, o2)
		default:
			apiCheck(false, "invalid option")
		}
	}
	return false
}

func ToInteger(l *State, index int) (int, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		return int(n), true
	}
	return 0, false
}

func ToUnsigned(l *State, index int) (uint, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		const supUnsigned = float64(^uint(0)) + 1
		return uint(n - math.Floor(n/supUnsigned)*supUnsigned), true
	}
	return 0, false
}

func ToString(l *State, index int) (string, bool) {
	v := l.indexToValue(index)
	if s, ok := v.(string); ok {
		return s, true
	}
	return toString(v)
}

func RawLength(l *State, index int) int {
	switch v := l.indexToValue(index).(type) {
	case string:
		return len(v)
	// case *userData:
	// 	return reflect.Sizeof(v.data)
	case *table:
		return v.length()
	}
	return 0
}

func ToGoFunction(l *State, index int) Function {
	switch v := l.indexToValue(index).(type) {
	case Function:
		return v
	case *goClosure:
		return v.function
	}
	return nil
}

func ToUserData(l *State, index int) interface{} {
	if d, ok := l.indexToValue(index).(*userData); ok {
		return d.data
	}
	return nil
}

func ToThread(l *State, index int) *State {
	if t, ok := l.indexToValue(index).(*State); ok {
		return t
	}
	return nil
}

func ToValue(l *State, index int) interface{} {
	v := l.indexToValue(index)
	switch v := v.(type) {
	case *table:
	case *luaClosure:
	case *goClosure:
	case Function:
	case *State:
	case *userData:
		return v.data
	default:
		return nil
	}
	return v
}

func PushString(l *State, s string) string { // TODO is it useful to return the argument?
	l.apiPush(s)
	return s
}

// this function handles only %d, %c, %f, %p, and %s formats
func PushFString(l *State, format string, args ...interface{}) string {
	n, i := 0, 0
	for {
		e := strings.IndexRune(format, '%')
		if e < 0 {
			break
		}
		l.checkStack(2) // format + item
		l.push(format[:e])
		switch format[e+1] {
		case 's':
			if args[i] == nil {
				l.push("(null)")
			} else {
				l.push(args[i].(string))
			}
			i++
		case 'c':
			l.push(string(args[i].(rune)))
			i++
		case 'd':
			l.push(float64(args[i].(int)))
			i++
		case 'f':
			l.push(args[i].(float64))
			i++
		case 'p':
			l.push(fmt.Sprintf("%p", args[i]))
		case '%':
			l.push("%")
		default:
			l.runtimeError("invalid option " + format[e:e+2] + " to 'lua_pushfstring'")
		}
		n += 2
		format = format[e+2:]
	}
	l.checkStack(1)
	l.push(format)
	if n > 0 {
		l.concat(n + 1)
	}
	return l.stack[l.top-1].(string)
}

func PushGoClosure(l *State, function Function, n int) {
	if n == 0 {
		l.apiPush(function)
	} else {
		l.checkElementCount(n)
		apiCheck(n <= maxUpValue, "upvalue index too large")
		cl := &goClosure{function: function, upValues: make([]value, n)}
		l.top -= n
		copy(cl.upValues, l.stack[l.top:l.top+n])
		l.apiPush(cl)
	}
}

func PushThread(l *State) bool {
	l.apiPush(l)
	return l.global.mainThread == l
}

func Global(l *State, name string) {
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.stack[l.top-1] = l.tableAt(g, l.stack[l.top-1])
}

func Field(l *State, index int, name string) {
	t := l.indexToValue(index)
	l.apiPush(name)
	l.stack[l.top-1] = l.tableAt(t, l.stack[l.top-1])
}

func RawGet(l *State, index int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.stack[l.top-1] = t.at(l.stack[l.top-1])
}

func RawGetInt(l *State, index, key int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.apiPush(t.atInt(key))
}

func RawGetValue(l *State, index int, p interface{}) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.apiPush(t.at(p))
}

func CreateTable(l *State, arrayCount, recordCount int) {
	l.apiPush(newTableWithSize(arrayCount, recordCount))
}

func MetaTable(l *State, index int) bool {
	var mt *table
	switch v := l.indexToValue(index).(type) {
	case *table:
		mt = v.metaTable
	case *userData:
		mt = v.metaTable
	default:
		mt = l.global.metaTable(v)
	}
	if mt == nil {
		return false
	}
	l.apiPush(mt)
	return true
}

func UserValue(l *State, index int) {
	d, ok := l.indexToValue(index).(*userData)
	apiCheck(ok, "userdata expected")
	l.apiPush(d.env)
}

func SetGlobal(l *State, name string) {
	l.checkElementCount(1)
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.setTableAt(g, l.stack[l.top-1], l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

func RawSet(l *State, index int) {
	l.checkElementCount(2)
	t, ok := l.stack[index].(*table)
	apiCheck(ok, "table expected")
	t.put(l.stack[l.top-2], l.stack[l.top-1])
	t.invalidateTagMethodCache()
	l.top -= 2
}

func SetUserValue(l *State, index int) {
	l.checkElementCount(1)
	d, ok := l.indexToValue(index).(*userData)
	apiCheck(ok, "userdata expected")
	if l.stack[l.top-1] == nil {
		d.env = nil
	} else {
		t, ok := l.stack[l.top-1].(*table)
		apiCheck(ok, "table expected")
		d.env = t
	}
	l.top--
}

func SetMetaTable(l *State, index int) {
	l.checkElementCount(1)
	mt, ok := l.stack[l.top-1].(*table)
	apiCheck(ok || l.stack[l.top-1] == nil, "table expected")
	switch v := l.indexToValue(index).(type) {
	case *table:
		v.metaTable = mt
	case *userData:
		v.metaTable = mt
	default:
		l.global.metaTables[TypeOf(l, index)] = mt
	}
	l.top--
}

func Error(l *State) {
	l.checkElementCount(1)
	l.errorMessage()
}

func Next(l *State, index int) bool {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	if l.next(t, l.top-1) {
		l.apiIncrementTop()
		return true
	}
	// no more elements
	l.top-- // remove key
	return false
}

func Concat(l *State, n int) {
	l.checkElementCount(n)
	if n >= 2 {
		l.concat(n)
	} else if n == 0 { // push empty string
		l.apiPush("")
	} // else n == 1; nothing to do
}

func Register(l *State, name string, f Function) {
	PushGoFunction(l, f)
	SetGlobal(l, name)
}

func (l *State) setErrorObject(status Status, oldTop int) {
	switch status {
	case MemoryError:
		l.stack[oldTop] = l.global.memoryErrorMessage
	case ErrorError:
		l.stack[oldTop] = "error in error handling"
	default:
		l.stack[oldTop] = l.stack[l.top-1]
	}
	l.top = oldTop + 1
}

func (l *State) protectedCall(f func(), oldTop, errorFunc int) Status {
	callInfo, allowHook, nonYieldableCallCount, errorFunction := l.callInfo, l.allowHook, l.nonYieldableCallCount, l.errorFunction
	l.errorFunction = errorFunc
	status := l.protect(f)
	if status != Ok {
		l.close(oldTop)
		l.setErrorObject(status, oldTop)
		l.callInfo, l.allowHook, l.nonYieldableCallCount = callInfo, allowHook, nonYieldableCallCount
		// l.shrinkStack()
	}
	l.errorFunction = errorFunction
	return status
}

func UpValue(l *State, function, index int) (name string, ok bool) {
	var v value
	switch f := l.indexToValue(function).(type) {
	case *goClosure:
		if 1 <= index && index <= f.upValueCount() {
			v, ok = f.upValue(index-1), true
		}
	case *luaClosure:
		if 1 <= index && index <= f.upValueCount() {
			name, v, ok = f.prototype.upValues[index-1].name, f.upValue(index-1), true
		}
	}
	if ok {
		l.apiPush(v)
	}
	return
}

func SetUpValue(l *State, function, index int) (name string, ok bool) {
	switch f := l.indexToValue(function).(type) {
	case *goClosure:
		if 1 <= index && index <= f.upValueCount() {
			ok = true
			l.top--
			f.setUpValue(index-1, l.stack[l.top])
		}
	case *luaClosure:
		if 1 <= index && index <= f.upValueCount() {
			name, ok = f.prototype.upValues[index-1].name, true
			l.top--
			f.setUpValue(index-1, l.stack[l.top])
		}
	}
	return
}

func Top(l *State) int                             { return l.top - (l.callInfo.function() + 1) }
func Call(l *State, argCount, resultCount int)     { CallWithContinuation(l, argCount, resultCount, 0, nil) }
func Copy(l *State, from, to int)                  { l.move(to, l.indexToValue(from)) }
func Version(l *State) *float64                    { return l.global.version }
func UpValueIndex(i int) int                       { return RegistryIndex - i }
func isPseudoIndex(i int) bool                     { return i <= RegistryIndex }
func ApiCheckStackSpace(l *State, n int)           { l.assert(n < l.top-l.callInfo.function()) }
func TypeName(l *State, t Type) string             { return typeNames[t+1] }
func ToNumber(l *State, index int) (float64, bool) { return toNumber(l.indexToValue(index)) }
func ToBoolean(l *State, index int) bool           { return !isFalse(l.indexToValue(index)) }
func Table(l *State, index int)                    { l.stack[l.top-1] = l.tableAt(l.indexToValue(index), l.stack[l.top-1]) }
func PushValue(l *State, index int)                { l.apiPush(l.indexToValue(index)) }
func PushNil(l *State)                             { l.apiPush(nil) }
func PushNumber(l *State, n float64)               { l.apiPush(n) }
func PushInteger(l *State, n int)                  { l.apiPush(float64(n)) }
func PushUnsigned(l *State, n uint)                { l.apiPush(float64(n)) }
func PushBoolean(l *State, b bool)                 { l.apiPush(b) }
func PushLightUserData(l *State, d interface{})    { l.apiPush(d) }
func PushUserData(l *State, d interface{})         { l.apiPush(&userData{data: d}) }
func Length(l *State, index int)                   { l.apiPush(l.objectLength(l.indexToValue(index))) }
func Pop(l *State, n int)                          { SetTop(l, -n-1) }
func NewTable(l *State)                            { CreateTable(l, 0, 0) }
func PushGoFunction(l *State, f Function)          { PushGoClosure(l, f, 0) }
func IsFunction(l *State, index int) bool          { return TypeOf(l, index) == TypeFunction }
func IsTable(l *State, index int) bool             { return TypeOf(l, index) == TypeTable }
func IsLightUserData(l *State, index int) bool     { return TypeOf(l, index) == TypeLightUserData }
func IsNil(l *State, index int) bool               { return TypeOf(l, index) == TypeNil }
func IsBoolean(l *State, index int) bool           { return TypeOf(l, index) == TypeBoolean }
func IsThread(l *State, index int) bool            { return TypeOf(l, index) == TypeThread }
func IsNone(l *State, index int) bool              { return TypeOf(l, index) == TypeNone }
func IsNoneOrNil(l *State, index int) bool         { return TypeOf(l, index) <= TypeNil }
func PushGlobalTable(l *State)                     { RawGetInt(l, RegistryIndex, RegistryIndexGlobals) }
