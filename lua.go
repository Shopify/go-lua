package lua

import (
	"errors"
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

var (
	Yield        = errors.New("yield")
	RuntimeError = errors.New("runtime error")
	SyntaxError  = errors.New("syntax error")
	MemoryError  = errors.New("memory error")
	GCError      = errors.New("garbage collection error")
	ErrorError   = errors.New("error within the error handler")
	FileError    = errors.New("file error")
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

type Operator int

const (
	OpAdd Operator = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPow
	OpUnaryMinus
)

type CmpOperator int

const (
	OpEq CmpOperator = iota
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

// Set functions (stack -> Lua)
// RawSetValue(index int, p interface{})

// Debug API
// Local(activationRecord *Debug, index int) string
// SetLocal(activationRecord *Debug, index int) string
// UpValueId(function, index int) interface{}
// UpValueJoin(function1, index1, function2, index2 int)
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
	status                error
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

// Context is called y a continuation function to retrieve the status of the
// thread and a context information.  When called in the origin function, it
// will always return (nil, 0). When called inside a continuation function, it
// will return (Yield, ctx), where `ctx` is the value that was passed to the
// callee together with the continuation function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getctx
func Context(l *State) (error, int) {
	if l.callInfo.isCallStatus(callStatusYielded) {
		callInfo := l.callInfo.(*goCallInfo)
		return callInfo.status, callInfo.context
	}
	return nil, 0
}

// CallWithContinuation is exactly like Call, but allows the called function to
// yield.
//
// http://www.lua.org/manual/5.2/manual.html#lua_callk
func CallWithContinuation(l *State, argCount, resultCount, context int, continuation Function) {
	apiCheck(continuation == nil || !l.callInfo.isLua(), "cannot use continuations inside hooks")
	l.checkElementCount(argCount + 1)
	apiCheck(l.status == nil, "cannot do calls on non-normal thread")
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

// ProtectedCall calls a function in protected mode.  Both `argCount` and
// `resultCount` have the same meaning as in Call.  If there are no errors
// during the call, ProtectedCall behaves exactly like Call.
//
// However, if there is any error, ProtectedCall catches it, pushes a single
// value on the stack (the error message), and returns an error. Like Call,
// ProtectedCall always removes the function and its arguments from the stack.
//
// If `errorFunction` is 0, then the error message returned on the stack is
// exactly the original error message.  Otherwise, errorFunction is the stack
// index of an error handler (in the Lua C, message handler).  This cannot be
// a pseudo-index in the current implementation.  In case of runtime errors,
// this function will be called with the error message and its return value
// will be the message returned on the stack by ProtectedCall.
//
// Typically, the error handler is used to add more debug information to the
// error message, such as a stack traceback.  Such information cannot be
// gathered after the return of ProtectedCall, since by then, the stack has
// unwound.
//
// The possible errors are the following:
//
//    RuntimeError  a runtime error
//    MemoryError   allocating memory, the error handler is not called
//    GCError       running a __gc metamethod (usually unrelated to the call)
//    ErrorError    running the error handler
//
// http://www.lua.org/manual/5.2/manual.html#lua_pcall
func ProtectedCall(l *State, argCount, resultCount, errorFunction int) error {
	return ProtectedCallWithContinuation(l, argCount, resultCount, errorFunction, 0, nil)
}

// ProtectedCallWithContinuation behaves exactly like ProtectedCall, but
// allows the called function to yield.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pcallk
func ProtectedCallWithContinuation(l *State, argCount, resultCount, errorFunction, context int, continuation Function) error {

	apiCheck(continuation == nil || !l.callInfo.isLua(), "cannot use continuations inside hooks")
	l.checkElementCount(argCount + 1)
	apiCheck(l.status == nil, "cannot do calls on non-normal thread")
	l.checkResults(argCount, resultCount)
	if errorFunction != 0 {
		apiCheckStackIndex(errorFunction, l.indexToValue(errorFunction))
		errorFunction = AbsIndex(l, errorFunction)
	}

	f := l.top - (argCount + 1)

	defer l.adjustResults(resultCount)

	if continuation == nil || l.nonYieldableCallCount > 0 {
		return l.protectedCall(func() { l.call(f, resultCount, false) }, f, errorFunction)
	}

	c := l.callInfo.(*goCallInfo)
	c.continuation, c.context, c.extra, c.oldAllowHook, c.oldErrorFunction = continuation, context, f, l.allowHook, l.errorFunction
	l.errorFunction = errorFunction
	c.setCallStatus(callStatusYieldableProtected)
	l.call(f, resultCount, true)
	c.clearCallStatus(callStatusYieldableProtected)
	l.errorFunction = c.oldErrorFunction

	return nil
}

// Load loads a Lua chunk, without running it. If there are no errors, it
// pushes the compiled chunk as a Lua function on top of the stack.
// Otherwise, it pushes an error message.
//
// http://www.lua.org/manual/5.2/manual.html#lua_load
func Load(l *State, r io.Reader, chunkName string, mode Mode) error {
	if chunkName == "" {
		chunkName = "?"
	}

	err := protectedParser(l, r, chunkName, mode)
	if err != nil {
		return err
	}

	if f := l.stack[l.top-1].(*luaClosure); f.upValueCount() == 1 {
		f.setUpValue(0, l.global.registry.atInt(RegistryIndexGlobals))
	}
	return nil
}

// NewState creates a new thread running in a new, independant state.
//
// http://www.lua.org/manual/5.2/manual.html#lua_newstate
func NewState() *State {
	v := float64(VersionNumber)
	l := &State{allowHook: true, status: nil, nonYieldableCallCount: 1}
	g := &globalState{mainThread: l, registry: newTable(), version: &v, memoryErrorMessage: "not enough memory"}
	l.global = g
	l.initializeStack()
	g.registry.putAtInt(RegistryIndexMainThread, l)
	g.registry.putAtInt(RegistryIndexGlobals, newTable())
	copy(g.tagMethodNames[:], eventNames)
	return l
}

func apiCheckStackIndex(index int, v value) {
	apiCheck(v != nil && !isPseudoIndex(index), fmt.Sprintf("index %d not in the stack", index))
}

// SetField does the equivalent of `table[key]=v`, where `table` is the value
// at `index` and `v` is the value on top of the stack.
//
// This function pops the value from the stack. As in Lua, this function may
// trigger a metamethod for the "newindex" event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setfield
func SetField(l *State, index int, key string) {
	l.checkElementCount(1)
	t := l.indexToValue(index)
	l.push(key)
	l.setTableAt(t, key, l.stack[l.top-2])
	l.top -= 2
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

// AbsIndex converts the acceptable index `index` to an absolute index (that
// is, one that does not depend on the stack top).
//
// http://www.lua.org/manual/5.2/manual.html#lua_absindex
func AbsIndex(l *State, index int) int {
	if index > 0 || isPseudoIndex(index) {
		return index
	}
	return l.top - l.callInfo.function() + index
}

// SetTop accepts any index, or 0, and sets the stack top to `index`.  If the
// new top is larger than the old one, then the new elements are filled with
// Nil.  If `index` is 0, then all stack elements are removed.
//
// If `index` is negative, the stack will be decremented by that much.  If
// the decrement is larger than the stack, SetTop will panic().
//
// http://www.lua.org/manual/5.2/manual.html#lua_settop
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

// Remove the element at the given valid `index`, shifting down the elements
// above `index` to fill the gap.  This function cannot be called with a
// pseudo-index, because a pseudo-index is not an actual stack position.
//
// http://www.lua.org/manual/5.2/manual.html#lua_remove
func Remove(l *State, index int) {
	apiCheckStackIndex(index, l.indexToValue(index))
	i := AbsIndex(l, index)
	copy(l.stack[i:l.top-1], l.stack[i+1:l.top])
	l.top--
}

// Insert moves the top element into the given valid index, shifting up the
// elements above this index to open space.  This function cannot be called
// with a pseudo-index, because a pseudo-index is not an actual stack position.
//
// http://www.lua.org/manual/5.2/manual.html#lua_insert
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

// Replace moves the top element into the given valid `index` without shifting
// any element (therefore replacing the value at the given index), and then
// pops the top element.
//
// http://www.lua.org/manual/5.2/manual.html#lua_replace
func Replace(l *State, index int) {
	l.checkElementCount(1)
	l.move(index, l.stack[l.top-1])
	l.top--
}

// CheckStack ensures that there are at least `size` free stack slots in the
// stack.  This call will not panic(), unlike the other CheckXxx() functions.
//
// http://www.lua.org/manual/5.2/manual.html#lua_checkstack
func CheckStack(l *State, size int) bool {
	callInfo := l.callInfo
	ok := l.stackLast-l.top > size
	if !ok && l.top+extraStack <= maxStack-size {
		ok = l.protect(func() { l.growStack(size) }) == nil
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

// TypeOf returns the type of the value at `index`, or TypeNone for a non-
// valid (but acceptable) index.
//
// http://www.lua.org/manual/5.2/manual.html#lua_type
func TypeOf(l *State, index int) Type {
	return l.valueToType(l.indexToValue(index))
}

// IsGoFunction verifies that the value at `index` is a Go function. Similar to `lua_iscfunction`.
//
// http://www.lua.org/manual/5.2/manual.html#lua_iscfunction
func IsGoFunction(l *State, index int) bool {
	if _, ok := l.indexToValue(index).(Function); ok {
		return true
	}
	_, ok := l.indexToValue(index).(*goClosure)
	return ok
}

// IsNumber verifies that the value at `index` is a number.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnumber
func IsNumber(l *State, index int) bool {
	_, ok := toNumber(l.indexToValue(index))
	return ok
}

// IsString verifies that the value at `index` is a string, or a number (which
// is always convertible to a string).
//
// http://www.lua.org/manual/5.2/manual.html#lua_isstring
func IsString(l *State, index int) bool {
	if _, ok := l.indexToValue(index).(string); ok {
		return true
	}
	_, ok := l.indexToValue(index).(float64)
	return ok
}

// IsUserData verifies that the value at `index` is a userdata (light or full).
//
// http://www.lua.org/manual/5.2/manual.html#lua_isuserdata
func IsUserData(l *State, index int) bool {
	_, ok := l.indexToValue(index).(*userData)
	return ok
}

// Arith performs an arithmetic operation over the two values (or one, in
// case of negation) at the top of the stack, with the value at the top being
// the second operand, ops these values and pushes the result of the operation.
// The function follows the semantics of the corresponding Lua operator
// (that is, it may call metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_arith
func Arith(l *State, op Operator) {
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

// RawEqual verifies that the values at `index1` and `index2` are primitively
// equal (that is, without calling their metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_raqequal
func RawEqual(l *State, index1, index2 int) (bool, error) {
	o1, o2 := l.indexToValue(index1), l.indexToValue(index2)
	if o1 != nil && o2 != nil {
		return o1 == o2, nil
	}

	if o1 == nil && o2 != nil {
		return false, fmt.Errorf("index1 (%d) doesn't exist on the stack", index1)
	}

	if o1 != nil && o2 == nil {
		return false, fmt.Errorf("index2 (%d) doesn't exist on the stack", index2)
	}

	return false, fmt.Errorf("both indices (%d and %d) don't exist on the stack",
		index1, index2)
}

// Compare compares two values.
//
// http://www.lua.org/manual/5.2/manual.html#lua_compare
func Compare(l *State, index1, index2 int, op CmpOperator) bool {
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

// ToInteger converts the Lua value at `index` into a signed integer.  The Lua
// value must be a number, or a string convertible to a number.
//
// If the number is not an integer, it is truncated in some non-specified way.
//
// If the operation failed, the first return value will be 0 and the second
// return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tointegerx
func ToInteger(l *State, index int) (int, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		return int(n), true
	}
	return 0, false
}

// ToUnsigned converts the Lua value at `index` to a Go `uint`. The Lua value
// must be a number or a string convertible to a number.
//
// If the number is not an unsigned integer, it is truncated in some non-
// specified way.  If the number is outside the range of uint, it is normalize
// to the remainder of its division by one more than the maximum representable
// value.
//
// If the operation failed, the first return value will be 0 and the second
// return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tounsignedx
func ToUnsigned(l *State, index int) (uint, bool) {
	if n, ok := toNumber(l.indexToValue(index)); ok {
		const supUnsigned = float64(^uint(0)) + 1
		return uint(n - math.Floor(n/supUnsigned)*supUnsigned), true
	}
	return 0, false
}

// ToString  converts the Lua value at `index` to a Go string.  The Lua value
// must also be a string or a number; otherwise the function returns an empty
// string and false for its second return value.
//
// If the value at `index` is a number, than THAT VALUE IS ALSO CHANGED
// to a string. This change will confuse Next when ToString is applied during
// a table traversal.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tolstring
func ToString(l *State, index int) (string, bool) {
	v := l.indexToValue(index)
	if s, ok := v.(string); ok {
		return s, true
	}
	return toString(v)
}

// RawLength returns the length of the value at `index`.  For strings, this is
// the length.  For tables, this is the result of the `#` operator with no
// metamethods.  For userdata, this is the size of the block of memory
// allocated for the userdata (not implemented yet). For other values, it is 0.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawlen
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

// ToGoFunction converts a value at `index` into a Go function.  That value
// must be a Go function, otherwise it returns nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tocfunction
func ToGoFunction(l *State, index int) Function {
	switch v := l.indexToValue(index).(type) {
	case Function:
		return v
	case *goClosure:
		return v.function
	}
	return nil
}

// ToUserData returns an interface{} of the userdata of the value at `index`.
// Otherwise, it returns nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_touserdata
func ToUserData(l *State, index int) interface{} {
	if d, ok := l.indexToValue(index).(*userData); ok {
		return d.data
	}
	return nil
}

// ToThread converts the value at `index` to a Lua thread (a State). This
// value must be a thread, otherwise the return value will be nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tothread
func ToThread(l *State, index int) *State {
	if t, ok := l.indexToValue(index).(*State); ok {
		return t
	}
	return nil
}

// ToValue convertes the value at `index` into a generic Go interface{}.  The
// value can be a userdata, a table, a thread, or a function.  Otherwise, the
// function returns nil.
//
// Different objects will give different values.  There is no way to convert
// the value back into its original value.
//
// Typically, this function is used only for debug information.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tovalue
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

// PushString pushes a string onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushstring
func PushString(l *State, s string) string { // TODO is it useful to return the argument?
	l.apiPush(s)
	return s
}

// PushFString pushes onto the stack a formatted string and returns that
// string.  It is similar to fmt.Sprintf, but has some differences: the
// conversion specifiers are quite restricted.  There are no flags, widths,
// or precisions.  The conversion specifiers can only be `%%` (inserts a `%`
// in the string), `%s`, `%f` (a Lua number), `%p` (a pointer as a hexadecimal
// numeral), `%d` and `%c` (an integer as a byte).
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushfstring
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

// PushGoClosure pushes a new Go closure onto the stack.
//
// When a Go function is created, it is possible to associate some values with
// it, thus creating a Go closure; these values are then accessible to the
// function whenever it is called.  To associate values with a C function,
// first these values should be pushed onto the stack (when there are multiple
// values, the first value is pushed first).  Then PushGoClosure is called to
// create and push the Go function onto the stack, with the argument `n`
// telling how many values should be associated with the function.  Calling
// PushGoClosure also pops these values from the stack.
//
// When `n` is 0, this function creates a light Go function, which is just a
// Go function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushcclosure
func PushGoClosure(l *State, function Function, n uint8) {
	if n == 0 {
		l.apiPush(function)
		return
	}
	nInt := int(n)

	l.checkElementCount(nInt)
	cl := &goClosure{function: function, upValues: make([]value, n)}
	l.top -= nInt
	copy(cl.upValues, l.stack[l.top:l.top+nInt])
	l.apiPush(cl)
}

// PushThread pushes the thread `l` onto the stack.  It returns true if `l` is
// the main thread of its state.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushthread
func PushThread(l *State) bool {
	l.apiPush(l)
	return l.global.mainThread == l
}

// Global pushes onto the stack the value of the global `name`.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getglobal
func Global(l *State, name string) {
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.stack[l.top-1] = l.tableAt(g, l.stack[l.top-1])
}

// Field pushes onto the stack the value `table[name]`, where `table` is the
// table at on the stack at the given `index`. This call may trigger a
// metamethod for the `index` event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getfield
func Field(l *State, index int, name string) {
	t := l.indexToValue(index)
	l.apiPush(name)
	l.stack[l.top-1] = l.tableAt(t, l.stack[l.top-1])
}

// RawGet is similar to GetTable, but does a raw access (without metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawget
func RawGet(l *State, index int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.stack[l.top-1] = t.at(l.stack[l.top-1])
}

// RawGetInt pushes onto the stack the value `table[key]` where `table` is the
// value at `index` on the stack. The access is raw, as it doesn't invoke
// metamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawgeti
func RawGetInt(l *State, index, key int) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.apiPush(t.atInt(key))
}

// RawGetValue pushes onto the stack value `table[p]` where `table` is the
// value at `index` on the stack, and `p` is a light userdata.  The access is
// raw, as it doesn't invoke metamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawgetp
func RawGetValue(l *State, index int, p interface{}) {
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	l.apiPush(t.at(p))
}

// CreateTable creates a new empty table and pushes it onto the stack.
// `arrayCount` is a hint for how many elements the table will have as a
// sequence; `recordCount` is a hint for how many other elements the table
// will have.  Lua may use these hints to preallocate memory for the the new
// table.  This pre-allocation is useful for performance when you know in
// advance how many elements the table will have.  Otherwise, you can use the
// function NewTable.
//
// http://www.lua.org/manual/5.2/manual.html#lua_createtable
func CreateTable(l *State, arrayCount, recordCount int) {
	l.apiPush(newTableWithSize(arrayCount, recordCount))
}

// MetaTable pushes onto the stack the metatable of the value at `index`.  If
// the value at `index` does not have a metatable, the function returns
// `false` and nothing is put onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getmetatable
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

// UserValue pushes onto the stack the Lua value associated with the userdata
// at `index`.  This value must be a table or nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getuservalue
func UserValue(l *State, index int) {
	d, ok := l.indexToValue(index).(*userData)
	apiCheck(ok, "userdata expected")
	l.apiPush(d.env)
}

// SetGlobal pops a value from the stack and sets it as the new value of
// global name.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setglobal
func SetGlobal(l *State, name string) {
	l.checkElementCount(1)
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.setTableAt(g, l.stack[l.top-1], l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

// SetTable does the equivalent of `table[key]=v`m where `table` is the value
// at `index`, `v` is the value at the top of the stack and `key` is the value
// just below the top.
//
// The function pops both the key and the value from the stack.  As in Lua,
// this function may trigger a metamethod for the "newindex" event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_settable
func SetTable(l *State, index int) {
	l.checkElementCount(2)
	l.setTableAt(l.indexToValue(index), l.stack[l.top-2], l.stack[l.top-1])
	l.top -= 2
}

// RawSet is similar to SetTable, but does a raw assignment (without
// metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawset
func RawSet(l *State, index int) {
	l.checkElementCount(2)
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	t.put(l.stack[l.top-2], l.stack[l.top-1])
	t.invalidateTagMethodCache()
	l.top -= 2
}

// RawSetInt does the equivalent of `table[n]=v` where `table` is the table at
// `index` and `v` is the value at the top of the stack.
//
// This function pops the value from the stack.  The assignment is raw; it
// doesn't invoke methamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawseti
func RawSetInt(l *State, index, key int) {
	l.checkElementCount(1)
	t, ok := l.indexToValue(index).(*table)
	apiCheck(ok, "table expected")
	t.putAtInt(key, l.stack[l.top-1])
	l.top--
}

// SetUserValue pops a table or Nil from the stack and sets it as the new
// value associated to the userdata at `index`.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setuservalue
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

// SetMetaTable pops a table from the stack and sets it as the new metatable
// for the value at `index.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setmetatable
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

// Error generates a Lua error.  The error message must be on the stack top.
// The error can be any of any Lua type. This function panic().
//
// http://www.lua.org/manual/5.2/manual.html#lua_error
func Error(l *State) {
	l.checkElementCount(1)
	l.errorMessage()
}

// Next pops a key from the stack and pushes a key-value pair from the table
// at `index`, while the table has next elements.  If there are no more
// elements, nothing is pushed on the stack and Next returns false.
//
// While traversing a table, DO NOT call ToString directly on a key, unless
// you know that the key is actually a string.  Recall that ToString may
// change the value at the given index; this confuses the next call to Next.
//
// http://www.lua.org/manual/5.2/manual.html#lua_next
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

// Concat concatenates the `n` values at the top of the stack, pops them, and
// leaves the result at the top.  If `n` is 1, the result is the single value
// on the stack (that is, the function does nothing); if `n` is 0, the result
// is the empty string. Concatenation is performed following the usual
// semantic of Lua.
//
// http://www.lua.org/manual/5.2/manual.html#lua_concat
func Concat(l *State, n int) {
	l.checkElementCount(n)
	if n >= 2 {
		l.concat(n)
	} else if n == 0 { // push empty string
		l.apiPush("")
	} // else n == 1; nothing to do
}

// Register sets the Go function `f` as the new value of global `name`.  If
// `name` was already defined, it is overwritten.
//
// http://www.lua.org/manual/5.2/manual.html#lua_register
func Register(l *State, name string, f Function) {
	PushGoFunction(l, f)
	SetGlobal(l, name)
}

func (l *State) setErrorObject(status error, oldTop int) {
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

func (l *State) protectedCall(f func(), oldTop, errorFunc int) error {
	callInfo, allowHook, nonYieldableCallCount, errorFunction := l.callInfo, l.allowHook, l.nonYieldableCallCount, l.errorFunction
	l.errorFunction = errorFunc

	status := l.protect(f)
	if status != nil {
		l.close(oldTop)
		l.setErrorObject(status, oldTop)
		l.callInfo, l.allowHook, l.nonYieldableCallCount = callInfo, allowHook, nonYieldableCallCount
		// l.shrinkStack()
	}
	l.errorFunction = errorFunction
	return status
}

// UpValue returns the name of the upvalue at `index` away from `function`,
// where `index` cannot be greater than the number of upvalues.
//
// Returns an empty string and false if the index is greater than the number
// of upvalues.
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

// SetUpValue sets the value of a closure's upvalue. It assigns the value at
// the top of the stack to the upvalue and returns its name.  It also value
// from the stack. `function` and `index` are as in UpValue.
//
// Returns an empty string and false if the index is greater than the number
// of upvalues.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setupvalue
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

// Call calls a function. To do so, use the following protocol: first, the
// function to be called is pushed onto the stack; then, the arguments to the
// function are pushed in direct order; that is, the first argument is pushed
// first.  Finally, you call Call.  `argCount` is the number of arguments that
// you pushed onto the stack. All arguments and the function value are popped
// from the stack when the function is called.
//
// The resuls are pushed onto the stack when the function returns.  The number of results is adjusted to `resultCount`, unless `resultCount` is
// `MultipleReturns`.  In this case, all results from the function are
// pushed.  Lua takes care that the returned values fit into the stack space.
// The function results are pushed onto the stack in direct order (the first
// result is pushed first), so that after the call the last result is on the
// top of the stack.
//
// Any error inside the called function provokes a call to panic().
//
// http://www.lua.org/manual/5.2/manual.html#lua_call
func Call(l *State, argCount, resultCount int) { CallWithContinuation(l, argCount, resultCount, 0, nil) }

// Top returns the index of the top element in the stack. Because Lua indices
// start at 1, this result is equal to the number of elements in the stack (
// and so 0 means an empty stack).
//
// http://www.lua.org/manual/5.2/manual.html#lua_gettop
func Top(l *State) int { return l.top - (l.callInfo.function() + 1) }

// Copy moves the element at the index `from` into the valid index `to`
// without shifting any element (therefore replacing the value at that
// position).
//
// http://www.lua.org/manual/5.2/manual.html#lua_copy
func Copy(l *State, from, to int) { l.move(to, l.indexToValue(from)) }

// Version returns the address of the version number stored in the Lua core.
//
// http://www.lua.org/manual/5.2/manual.html#lua_version
func Version(l *State) *float64 { return l.global.version }

// UpValueIndex returns the pseudo-index that represents the i-th upvalue of
// the running function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_upvalueindex
func UpValueIndex(i int) int   { return RegistryIndex - i }
func isPseudoIndex(i int) bool { return i <= RegistryIndex }

// ApiCheckStackSpace verifies that the stack has at least `n` free stack
// slots in the stack, and panic() otherwise.
func ApiCheckStackSpace(l *State, n int) { l.assert(n < l.top-l.callInfo.function()) }

// TypeName returns the name of Type `t`.
//
// http://www.lua.org/manual/5.2/manual.html#lua_typename
func TypeName(l *State, t Type) string { return typeNames[t+1] }

// ToNumber converts the Lua value at `index` to the Go type for a Lua number (
// float64).  The Lua value must be a number or a string convertible to a
// number.
//
// If the operation failed, the first return value will be 0 and the second
// return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tonumberx
func ToNumber(l *State, index int) (float64, bool) { return toNumber(l.indexToValue(index)) }

// ToBoolean converts the Lua value at `index` to a Go boolean.  Like all
// tests in Lua, the only true values are `true` booleans and `nil`.
// Otherwise, all other Lua values evaluate to false.
//
// To accept only actual boolean values, use the test IsBoolean.
//
// http://www.lua.org/manual/5.2/manual.html#lua_toboolean
func ToBoolean(l *State, index int) bool { return !isFalse(l.indexToValue(index)) }

// Table pushes onto the stack the value `table[top]`, where `table` is the
// value at `index`, and `top` is the value at the top of the stack. This
// function pops the key from the stack, putting the resulting value in its
// place.  As in Lua, this function may trigger a metamethod for the "index"
// event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_gettable
func Table(l *State, index int) { l.stack[l.top-1] = l.tableAt(l.indexToValue(index), l.stack[l.top-1]) }

// PushValue pushesa copy of the element at `index` onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushvalue
func PushValue(l *State, index int) { l.apiPush(l.indexToValue(index)) }

// PushNil pushes a nil value onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushnil
func PushNil(l *State) { l.apiPush(nil) }

// PushNumber pushes a number onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushnumber
func PushNumber(l *State, n float64) { l.apiPush(n) }

// PushInteger pushes `n` onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushinteger
func PushInteger(l *State, n int) { l.apiPush(float64(n)) }

// PushUnsigned pushes `n` onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushunsigned
func PushUnsigned(l *State, n uint) { l.apiPush(float64(n)) }

// PushBoolean pushes a boolean value with value `b` onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushboolean
func PushBoolean(l *State, b bool) { l.apiPush(b) }

// PushLightUserData pushes a light user data onto the stack.  Userdata
// represents Go values in Lua.  A light userdata is an interface{}.  It is
// only equal to itself or another Go value pointing to the same address.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushlightuserdata
func PushLightUserData(l *State, d interface{}) { l.apiPush(d) }

// PushUserData is similar to PushLightUserData, but pushes a full userdata
// onto the stack.
func PushUserData(l *State, d interface{}) { l.apiPush(&userData{data: d}) }

// Length of the value at `index`; it is equivalent to the `#` operator in
// Lua. The result is pushed on the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_len
func Length(l *State, index int) { l.apiPush(l.objectLength(l.indexToValue(index))) }

// Pop pops `n` elements from the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pop
func Pop(l *State, n int) { SetTop(l, -n-1) }

// NewTable creates a new empty table and pushes it onto the stack.  It is
// equivalent to CreateTable(l, 0, 0).
//
// http://www.lua.org/manual/5.2/manual.html#lua_newtable
func NewTable(l *State) { CreateTable(l, 0, 0) }

// PushGoFunction pushes a Function implemented in Go onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushcfunction
func PushGoFunction(l *State, f Function) { PushGoClosure(l, f, 0) }

// IsFunction verifies that the value at `index` is a function, either Go or
// Lua function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isfunction
func IsFunction(l *State, index int) bool { return TypeOf(l, index) == TypeFunction }

// IsTable verifies that the value at `index` is a table.
//
// http://www.lua.org/manual/5.2/manual.html#lua_istable
func IsTable(l *State, index int) bool { return TypeOf(l, index) == TypeTable }

// IsLightUserData verifies that the value at `index` is a light userdata.
//
// http://www.lua.org/manual/5.2/manual.html#lua_islightuserdata
func IsLightUserData(l *State, index int) bool { return TypeOf(l, index) == TypeLightUserData }

// IsNil verifies that the value at `index` is nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnil
func IsNil(l *State, index int) bool { return TypeOf(l, index) == TypeNil }

// IsBoolean verifies that the value at `index` is a boolean.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isboolean
func IsBoolean(l *State, index int) bool { return TypeOf(l, index) == TypeBoolean }

// IsThread verifies that the value at `index` is a thread.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isthread
func IsThread(l *State, index int) bool { return TypeOf(l, index) == TypeThread }

// IsNone verifies that the value at `index` is not valid.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnone
func IsNone(l *State, index int) bool { return TypeOf(l, index) == TypeNone }

// IsNoneOrNil verifies that the value at `index` is either nil or invalid.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnonornil.
func IsNoneOrNil(l *State, index int) bool { return TypeOf(l, index) <= TypeNil }

// PushGlobalTable pushes the global environment onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushglobaltable
func PushGlobalTable(l *State) { RawGetInt(l, RegistryIndex, RegistryIndexGlobals) }
