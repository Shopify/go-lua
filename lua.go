package lua

import (
	"fmt"
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
)

const (
	TypeNil = iota
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
	Version       = "Lua " + string(VersionMajor) + "." + string(VersionMinor)
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

type Hook func(state State, activationRecord *Debug)
type Function func(state State) int

type State interface {
	Version() *float64

	// basic stack manipulation
	AbsIndex(index int) int
	Top() int
	SetTop(index int)
	PushValue(index int)
	Remove(index int)
	Insert(index int)
	Replace(index int)
	Copy(from, to int)
	CheckStack(size int) bool
	// TODO XMove(from, to State, n int)

	// Access functions (stack -> Go)
	IsNumber(index int) bool
	IsString(index int) bool
	IsGoFunction(index int) bool
	IsUserData(index int) bool
	Type(index int) int
	TypeName(t int) string

	ToNumber(index int) (float64, bool)
	ToInteger(index int) (int, bool)
	ToUnsigned(index int) (uint, bool)
	ToBoolean(index int) bool
	ToString(index int) (string, bool)
	RawLength(index int) int
	ToGoFunction(index int) Function
	ToUserData(index int) interface{}
	ToThread(index int) State
	ToInterface(index int) interface{}

	// Comparison and arithmetic functions
	Arith(op int)
	RawEqual(index1, index2 int) bool
	Compare(index1, index2, op int) bool

	// Push functions (Go -> stack)
	PushNil()
	PushNumber(n float64)
	PushInteger(n int)
	PushUnsigned(n uint)
	PushString(s string) string
	PushFString(format string, args ...interface{}) string
	PushGoClosure(function Function, n int)
	PushBoolean(b bool)
	PushLightUserData(d interface{})
	PushThread() (isMainThread bool)

	// Get functions (Lua -> stack)
	Global(name string)
	Table(index int)
	Field(index int, name string)
	RawGet(index int)
	RawGetI(index, n int)
	RawGetP(index int, p interface{})
	CreateTable(arrayCount, recordCount int)
	// TODO NewUserData(?) interface{}
	MetaTable(index int) bool
	UserValue(index int)

	// Set functions (stack -> Lua)
	SetGlobal(name string)
	// SetTable(index int)
	// SetField(index int, name string)
	RawSet(index int)
	// RawSetI(index, n int)
	// RawSetP(index int, p interface{})
	SetMetaTable(index int)
	// SetUserValue(index int)

	CallWithContinuation(argCount, resultCount, context int, continuation Function)
	Call(argCount, resultCount int)

	RawGetInt(index, key int)
	SetField(index int, key string)
	ApiCheckStackSpace(n int)

	// Miscellaneous functions
	Error()
	Next(index int) bool
	Concat(n int)
	Length(index int)
	// TODO AllocateFunction() (f Alloc, userData interface{})
	// TODO SetAllocateFunction(f Alloc, userData interface{})

	// Useful functions
	Pop(n int)
	NewTable()
	Register(name string, f Function)
	PushGoFunction(f Function)
	IsFunction(index int) bool
	IsTable(index int) bool
	IsLightUserData(index int) bool
	IsNil(index int) bool
	IsBoolean(index int) bool
	IsThread(index int) bool
	IsNone(index int) bool
	IsNoneOrNil(index int) bool
	PushGlobalTable()

	// Debug API
	Stack(level int, activationRecord *Debug) bool
	Info(what string, activationRecord *Debug) bool
	// Local(activationRecord *Debug, index int) string
	// SetLocal(activationRecord *Debug, index int) string
	// UpValue(function, index int) string
	// SetUpValue(function, index int) string
	// UpValueId(function, index int) interface{}
	// UpValueJoin(function1, index1, function2, index2 int)
	// SetHook(function Hook, mask, count int) int // TODO bool?
	// Hook() Hook
	// HookMask() int
	// HookCount() int
}

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
type state struct {
	// TODO necessary? errorJmp *longjmp // current error recover point
	status                Status
	top                   int // first free slot in the stack
	global                *globalState
	callInfo              callInfo // call info for current function
	oldPC                 pc       // last pC traced
	stackLast             int      // last free slot in the stack
	stack                 []value
	nonYieldableCallCount uint
	nestedGoCallCount     uint
	hookMask              byte
	allowHook             bool
	baseHookCount         int
	hookCount             int
	hooker                Hook
	upValues              *openUpValue
	errorFunction         int         // current error handling function (stack index)
	baseCallInfo          luaCallInfo // callInfo for first level (go calling lua)
}

type globalState struct {
	mainThread     *state
	tagMethodNames [tmCount]string
	metaTables     [TypeCount]*table // metatables for basic types
	registry       *table
	// seed uint // randomized seed for hashes
	// upValueHead upValue // head of double-linked list of all open upvalues
	// panicFunction ? // to be called in unprotected errors
	version            *float64 // pointer to version number
	memoryErrorMessage string
}

func (g *globalState) metaTable(o value) *table {
	var t int
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
	case *state:
		t = TypeThread
	default:
		return nil
	}
	return g.metaTables[t]
}

func (l *state) ApiCheckStackSpace(n int) {
	l.assert(n < l.top-l.callInfo.function())
}

func (l *state) adjustResults(resultCount int) {
	if resultCount == MultipleReturns && l.callInfo.top() < l.top {
		l.callInfo.setTop(l.top)
	}
}

func apiCheck(condition bool, message string) {
	if !condition {
		panic(message)
	}
}

func (l *state) apiIncrementTop() {
	l.top++
	apiCheck(l.top <= l.callInfo.top(), "stack overflow")
}

func (l *state) apiPush(v value) {
	l.push(v)
	apiCheck(l.top <= l.callInfo.top(), "stack overflow")
}

func (l *state) checkElementCount(n int) {
	apiCheck(n < l.top-l.callInfo.function(), "not enough elements in the stack")
}

func (l *state) checkResults(argCount, resultCount int) {
	apiCheck(resultCount == MultipleReturns || l.callInfo.top()-l.top >= resultCount-argCount,
		"results from function overflow current stack size")
}

func (l *state) CallWithContinuation(argCount, resultCount, context int, continuation Function) {
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

func (l *state) Call(argCount, resultCount int) {
	l.CallWithContinuation(argCount, resultCount, 0, nil)
}

func (l *state) Version() *float64 {
	return l.global.version
}

func NewState() State {
	v := float64(VersionNumber)
	l := &state{allowHook: true, status: Ok, nonYieldableCallCount: 1}
	g := &globalState{mainThread: l, registry: newTable(), version: &v, memoryErrorMessage: "not enough memory"}
	l.global = g
	l.initializeStack()
	g.registry.putAtInt(RegistryIndexMainThread, l)
	g.registry.putAtInt(RegistryIndexGlobals, newTable())
	copy(g.tagMethodNames[:], eventNames)
	return l
}

func UpValueIndex(i int) int {
	return RegistryIndex - i
}

func isPseudoIndex(i int) bool {
	return i <= RegistryIndex
}

func (l *state) RawGetInt(index, key int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.apiPush(t.atInt(key))
}

func (l *state) SetField(index int, key string) {
	l.checkElementCount(1)
	t := l.indexToValue(index)
	l.stack[l.top] = key
	l.top++
	l.setTableAt(t, key, l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

func (l *state) indexToValue(index int) value {
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
		// if ttislcf(ci->func) { // light C function? TODO
		// 	return nil // it has no upvalues
		// }
		if closure := l.stack[callInfo.function()].(*goClosure); i <= len(closure.upValues) {
			return closure.upValues[i-1]
		}
		return nil
	}
}

func (l *state) AbsIndex(index int) int {
	if index > 0 || isPseudoIndex(index) {
		return index
	}
	return l.top - l.callInfo.function() + index
}

func (l *state) Top() int {
	return l.top - (l.callInfo.function() + 1)
}

func (l *state) SetTop(index int) {
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

func (l *state) PushValue(index int) {
	l.apiPush(l.indexToValue(index))
}

func (l *state) Remove(index int) {
	// TODO
}

func (l *state) Insert(index int) {
	// TODO
}

func (l *state) Replace(index int) {
	// TODO
}

func (l *state) Copy(from, to int) {
	// TODO
}

func (l *state) CheckStack(size int) bool {
	callInfo := l.callInfo
	ok := l.stackLast-l.top > size
	if !ok && l.top+extraStack <= maxStack-size {
		l.growStack(size) // TODO rawRunUnprotected?
		ok = true
	}
	if ok && callInfo.top() < l.top+size {
		callInfo.setTop(l.top + size)
	}
	return ok
}

func (l *state) Type(index int) int {
	switch l.indexToValue(index).(type) {
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
	case *state:
		return TypeThread
	}
	return TypeNone
}

func (l *state) TypeName(t int) string {
	return typeNames[t+1]
}

func (l *state) IsGoFunction(index int) bool {
	if _, ok := l.indexToValue(index).(Function); ok {
		return true
	}
	_, ok := l.indexToValue(index).(*goClosure)
	return ok
}

func (l *state) IsNumber(index int) bool {
	_, ok := toNumber(l.indexToValue(index))
	return ok
}

func (l *state) IsString(index int) bool {
	if _, ok := l.indexToValue(index).(string); ok {
		return true
	}
	_, ok := l.indexToValue(index).(float64)
	return ok
}

func (l *state) IsUserData(index int) bool {
	_, ok := l.indexToValue(index).(*userData)
	return ok
}

func (l *state) Arith(op int) {
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

func (l *state) RawEqual(index1, index2 int) bool {
	if o1, o2 := l.indexToValue(index1), l.indexToValue(index2); o1 != nil && o2 != nil {
		return o1 == o2
	}
	return false
}

func (l *state) Compare(index1, index2, op int) bool {
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

func (l *state) ToNumber(index int) (float64, bool) {
	return toNumber(l.indexToValue(index))
}

func (l *state) ToInteger(index int) (int, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		return int(n), true
	}
	return 0, false
}

func (l *state) ToUnsigned(index int) (uint, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		const supUnsigned = float64(^uint(0)) + 1
		return uint(n - math.Floor(n/supUnsigned)*supUnsigned), true
	}
	return 0, false
}

func (l *state) ToBoolean(index int) bool {
	return !isFalse(l.indexToValue(index))
}

func (l *state) ToString(index int) (string, bool) {
	v := l.indexToValue(index)
	if s, ok := v.(string); ok {
		return s, true
	}
	return toString(v)
}

func (l *state) RawLength(index int) int {
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

func (l *state) ToGoFunction(index int) Function {
	switch v := l.indexToValue(index).(type) {
	case Function:
		return v
	case *goClosure:
		return v.function
	}
	return nil
}

func (l *state) ToUserData(index int) interface{} {
	if d, ok := l.indexToValue(index).(*userData); ok {
		return d.data
	}
	return nil
}

func (l *state) ToThread(index int) State {
	if t, ok := l.indexToValue(index).(*state); ok {
		return t
	}
	return nil
}

func (l *state) ToInterface(index int) interface{} {
	v := l.indexToValue(index)
	switch v := v.(type) {
	case *table:
	case *luaClosure:
	case *goClosure:
	case Function:
	case *state:
	case *userData:
		return v.data
	default:
		return nil
	}
	return v
}

func (l *state) PushNil() {
	l.apiPush(nil)
}

func (l *state) PushNumber(n float64) {
	l.apiPush(n)
}

func (l *state) PushInteger(n int) {
	l.apiPush(float64(n))
}

func (l *state) PushUnsigned(n uint) {
	l.apiPush(float64(n))
}

func (l *state) PushString(s string) string { // TODO is it useful to return the argument?
	l.apiPush(s)
	return s
}

// this function handles only %d, %c, %f, %p, and %s formats
func (l *state) PushFString(format string, args ...interface{}) string {
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

func (l *state) PushGoClosure(function Function, n int) {
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

func (l *state) PushBoolean(b bool) {
	l.apiPush(b)
}

func (l *state) PushLightUserData(d interface{}) {
	l.apiPush(d)
}

func (l *state) PushThread() bool {
	l.apiPush(l)
	return l.global.mainThread == l
}

func (l *state) Global(name string) {
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.stack[l.top-1] = l.tableAt(g, l.stack[l.top-1])
}

func (l *state) Table(index int) {
	l.stack[l.top-1] = l.tableAt(l.indexToValue(index), l.stack[l.top-1])
}

func (l *state) Field(index int, name string) {
	t := l.indexToValue(index)
	l.apiPush(name)
	l.stack[l.top-1] = l.tableAt(t, l.stack[l.top-1])
}

func (l *state) RawGet(index int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.stack[l.top-1] = t.at(l.stack[l.top-1])
}

func (l *state) RawGetI(index, n int) {
	// TODO
}

func (l *state) RawGetP(index int, p interface{}) {
	// TODO
}

func (l *state) CreateTable(arrayCount, recordCount int) {
	l.apiPush(newTableWithSize(arrayCount, recordCount))
}

func (l *state) MetaTable(index int) bool {
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

func (l *state) UserValue(index int) {
	d, ok := l.indexToValue(index).(*userData)
	apiCheck(ok, "userdata expected")
	l.apiPush(d.env)
}

func (l *state) SetGlobal(name string) {
	l.checkElementCount(1)
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.setTableAt(g, l.stack[l.top-1], l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

func (l *state) RawSet(index int) {
	l.checkElementCount(2)
	t, ok := l.stack[index].(*table)
	apiCheck(ok, "table expected")
	t.put(l.stack[l.top-2], l.stack[l.top-1])
	t.invalidateTagMethodCache()
	l.top -= 2
}

func (l *state) SetMetaTable(index int) {
	l.checkElementCount(1)
	mt, ok := l.stack[l.top-1].(*table)
	apiCheck(ok || l.stack[l.top-1] == nil, "table expected")
	switch v := l.indexToValue(index).(type) {
	case *table:
		v.metaTable = mt
	case *userData:
		v.metaTable = mt
	default:
		l.global.metaTables[l.Type(index)] = mt
	}
	l.top--
}

func (l *state) Error() {
	l.checkElementCount(1)
	l.errorMessage()
}

func (l *state) Next(index int) bool {
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

func (l *state) Concat(n int) {
	l.checkElementCount(n)
	if n >= 2 {
		l.concat(n)
	} else if n == 0 { // push empty string
		l.apiPush("")
	} // else n == 1; nothing to do
}

func (l *state) Length(index int) {
	l.apiPush(l.objectLength(l.indexToValue(index)))
}

func (l *state) Pop(n int) {
	l.SetTop(-n - 1)
}

func (l *state) NewTable() {
	l.CreateTable(0, 0)
}

func (l *state) Register(name string, f Function) {
	l.PushGoFunction(f)
	l.SetGlobal(name)
}

func (l *state) PushGoFunction(f Function) {
	l.PushGoClosure(f, 0)
}

func (l *state) IsFunction(index int) bool {
	return l.Type(index) == TypeFunction
}

func (l *state) IsTable(index int) bool {
	return l.Type(index) == TypeTable
}

func (l *state) IsLightUserData(index int) bool {
	return l.Type(index) == TypeLightUserData
}

func (l *state) IsNil(index int) bool {
	return l.Type(index) == TypeNil
}

func (l *state) IsBoolean(index int) bool {
	return l.Type(index) == TypeBoolean
}

func (l *state) IsThread(index int) bool {
	return l.Type(index) == TypeThread
}

func (l *state) IsNone(index int) bool {
	return l.Type(index) == TypeNone
}

func (l *state) IsNoneOrNil(index int) bool {
	return l.Type(index) <= TypeNil
}

func (l *state) PushGlobalTable() {
	l.RawGetInt(RegistryIndex, RegistryIndexGlobals)
}
