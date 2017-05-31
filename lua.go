package lua

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

// MultipleReturns is the argument for argCount or resultCount in ProtectedCall and Call.
const MultipleReturns = -1

// Debug.Event and SetDebugHook mask argument values.
const (
	HookCall, MaskCall = iota, 1 << iota
	HookReturn, MaskReturn
	HookLine, MaskLine
	HookCount, MaskCount
	HookTailCall, MaskTailCall
)

// Errors introduced by the Lua VM.
var (
	SyntaxError = errors.New("syntax error")
	MemoryError = errors.New("memory error")
	ErrorError  = errors.New("error within the error handler")
	FileError   = errors.New("file error")
)

// A RuntimeError is an error raised internally by the Lua VM or through Error.
type RuntimeError string

func (r RuntimeError) Error() string { return "runtime error: " + string(r) }

// A Type is a symbolic representation of a Lua VM type.
type Type int

// Valid Type values.
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

// An Operator is an op argument for Arith.
type Operator int

// Valid Operator values for Arith.
const (
	OpAdd        Operator = iota // Performs addition (+).
	OpSub                        // Performs subtraction (-).
	OpMul                        // Performs multiplication (*).
	OpDiv                        // Performs division (/).
	OpMod                        // Performs modulo (%).
	OpPow                        // Performs exponentiation (^).
	OpUnaryMinus                 // Performs mathematical negation (unary -).
)

// A ComparisonOperator is an op argument for Compare.
type ComparisonOperator int

// Valid ComparisonOperator values for Compare.
const (
	OpEq ComparisonOperator = iota // Compares for equality (==).
	OpLT                           // Compares for less than (<).
	OpLE                           // Compares for less or equal (<=).
)

// Lua provides a registry, a predefined table, that can be used by any Go code
// to store whatever Lua values it needs to store. The registry table is always
// located at pseudo-index RegistryIndex, which is a valid index. Any Go
// library can store data into this table, but it should take care to choose
// keys that are different from those used by other libraries, to avoid
// collisions. Typically, you should use as key a string containing your
// library name, or a light userdata object in your code, or any Lua object
// created by your code. As with global names, string keys starting with an
// underscore followed by uppercase letters are reserved for Lua.
//
// The integer keys in the registry are used by the reference mechanism
// and by some predefined values. Therefore, integer keys should not be used
// for other purposes.
//
// When you create a new Lua state, its registry comes with some predefined
// values. These predefined values are indexed with integer keys defined as
// constants.
const (
	// RegistryIndex is the pseudo-index for the registry table.
	RegistryIndex = firstPseudoIndex

	// RegistryIndexMainThread is the registry index for the main thread of the
	// State. (The main thread is the one created together with the State.)
	RegistryIndexMainThread = iota

	// RegistryIndexGlobals is the registry index for the global environment.
	RegistryIndexGlobals
)

// Signature is the mark for precompiled code ('<esc>Lua').
const Signature = "\033Lua"

// MinStack is the minimum Lua stack available to a Go function.
const MinStack = 20

const (
	VersionMajor  = 5
	VersionMinor  = 2
	VersionNumber = 502
	VersionString = "Lua " + string('0'+VersionMajor) + "." + string('0'+VersionMinor)
)

// A RegistryFunction is used for arrays of functions to be registered by
// SetFunctions. Name is the function name and Function is the function.
type RegistryFunction struct {
	Name     string
	Function Function
}

// A Debug carries different pieces of information about a function or an
// activation record. Stack fills only the private part of this structure, for
// later use. To fill the other fields of a Debug with useful information, call
// Info.
type Debug struct {
	Event int

	// Name is a reasonable name for the given function. Because functions in
	// Lua are first-class values, they do not have a fixed name. Some functions
	// can be the value of multiple global variables, while others can be stored
	// only in a table field. The Info function checks how the function was
	// called to find a suitable name. If it cannot find a name, then Name is "".
	Name string

	// NameKind explains the name field. The value of NameKind can be "global",
	// "local", "method", "field", "upvalue", or "" (the empty string), according
	// to how the function was called. (Lua uses the empty string when no other
	// option seems to apply.)
	NameKind string

	// What is the string "Lua" if the function is a Lua function, "Go" if it is
	// a Go function, "main" if it is the main part of a chunk.
	What string

	// Source is the source of the chunk that created the function. If Source
	// starts with a '@', it means that the function was defined in a file where
	// the file name follows the '@'. If Source starts with a '=', the remainder
	// of its contents describe the source in a user-dependent manner. Otherwise,
	// the function was defined in a string where Source is that string.
	Source string

	// ShortSource is a "printable" version of source, to be used in error messages.
	ShortSource string

	// CurrentLine is the current line where the given function is executing.
	// When no line information is available, CurrentLine is set to -1.
	CurrentLine int

	// LineDefined is the line number where the definition of the function starts.
	LineDefined int

	// LastLineDefined is the line number where the definition of the function ends.
	LastLineDefined int

	// UpValueCount is the number of upvalues of the function.
	UpValueCount int

	// ParameterCount is the number of fixed parameters of the function (always 0
	// for Go functions).
	ParameterCount int

	// IsVarArg is true if the function is a vararg function (always true for Go
	// functions).
	IsVarArg bool

	// IsTailCall is true if this function invocation was called by a tail call.
	// In this case, the caller of this level is not in the stack.
	IsTailCall bool

	// callInfo is the active function.
	callInfo *callInfo
}

// A Hook is a callback function that can be registered with SetDebugHook to trace various VM events.
type Hook func(state *State, activationRecord Debug)

// A Function is a Go function intended to be called from Lua.
type Function func(state *State) int

// TODO XMove(from, to State, n int)
//
// Set functions (stack -> Lua)
// RawSetValue(index int, p interface{})
//
// Debug API
// Local(activationRecord *Debug, index int) string
// SetLocal(activationRecord *Debug, index int) string

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

// A State is an opaque structure representing per thread Lua state.
type State struct {
	error                 error
	shouldYield           bool
	top                   int // first free slot in the stack
	global                *globalState
	callInfo              *callInfo // call info for current function
	oldPC                 pc        // last pC traced
	stackLast             int       // last free slot in the stack
	stack                 []value
	nonYieldableCallCount int
	nestedGoCallCount     int
	hookMask              byte
	allowHook             bool
	internalHook          bool
	baseHookCount         int
	hookCount             int
	hooker                Hook
	upValues              *openUpValue
	errorFunction         int      // current error handling function (stack index)
	baseCallInfo          callInfo // callInfo for first level (go calling lua)
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
	case *goFunction:
		t = TypeFunction
	case closure:
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
	if resultCount == MultipleReturns && l.callInfo.top < l.top {
		l.callInfo.setTop(l.top)
	}
}

func (l *State) apiIncrementTop() {
	l.top++
	if apiCheck && l.top > l.callInfo.top {
		panic("stack overflow")
	}
}

func (l *State) apiPush(v value) {
	l.push(v)
	if apiCheck && l.top > l.callInfo.top {
		panic("stack overflow")
	}
}

func (l *State) checkElementCount(n int) {
	if apiCheck && n >= l.top-l.callInfo.function {
		panic("not enough elements in the stack")
	}
}

func (l *State) checkResults(argCount, resultCount int) {
	if apiCheck && resultCount != MultipleReturns && l.callInfo.top-l.top < resultCount-argCount {
		panic("results from function overflow current stack size")
	}
}

// Context is called by a continuation function to retrieve the status of the
// thread and context information. When called in the origin function, it
// will always return (0, false, nil). When called inside a continuation function,
// it will return (ctx, shouldYield, err), where ctx is the value that was
// passed to the callee together with the continuation function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getctx
func (l *State) Context() (int, bool, error) {
	if l.callInfo.isCallStatus(callStatusYielded) {
		return l.callInfo.context, l.callInfo.shouldYield, l.callInfo.error
	}
	return 0, false, nil
}

// CallWithContinuation is exactly like Call, but allows the called function to
// yield.
//
// http://www.lua.org/manual/5.2/manual.html#lua_callk
func (l *State) CallWithContinuation(argCount, resultCount, context int, continuation Function) {
	if apiCheck && continuation != nil && l.callInfo.isLua() {
		panic("cannot use continuations inside hooks")
	}
	l.checkElementCount(argCount + 1)
	if apiCheck && l.shouldYield {
		panic("cannot do calls on non-normal thread")
	}
	l.checkResults(argCount, resultCount)
	f := l.top - (argCount + 1)
	if continuation != nil && l.nonYieldableCallCount == 0 { // need to prepare continuation?
		l.callInfo.continuation = continuation
		l.callInfo.context = context
		l.call(f, resultCount, true) // just do the call
	} else { // no continuation or not yieldable
		l.call(f, resultCount, false) // just do the call
	}
	l.adjustResults(resultCount)
}

// ProtectedCall calls a function in protected mode. Both argCount and
// resultCount have the same meaning as in Call. If there are no errors
// during the call, ProtectedCall behaves exactly like Call.
//
// However, if there is any error, ProtectedCall catches it, pushes a single
// value on the stack (the error message), and returns an error. Like Call,
// ProtectedCall always removes the function and its arguments from the stack.
//
// If errorFunction is 0, then the error message returned on the stack is
// exactly the original error message. Otherwise, errorFunction is the stack
// index of an error handler (in the Lua C, message handler). This cannot be
// a pseudo-index in the current implementation. In case of runtime errors,
// this function will be called with the error message and its return value
// will be the message returned on the stack by ProtectedCall.
//
// Typically, the error handler is used to add more debug information to the
// error message, such as a stack traceback. Such information cannot be
// gathered after the return of ProtectedCall, since by then, the stack has
// unwound.
//
// The possible errors are the following:
//
//    RuntimeError  a runtime error
//    MemoryError   allocating memory, the error handler is not called
//    ErrorError    running the error handler
//
// http://www.lua.org/manual/5.2/manual.html#lua_pcall
func (l *State) ProtectedCall(argCount, resultCount, errorFunction int) error {
	return l.ProtectedCallWithContinuation(argCount, resultCount, errorFunction, 0, nil)
}

// ProtectedCallWithContinuation behaves exactly like ProtectedCall, but
// allows the called function to yield.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pcallk
func (l *State) ProtectedCallWithContinuation(argCount, resultCount, errorFunction, context int, continuation Function) (err error) {
	if apiCheck && continuation != nil && l.callInfo.isLua() {
		panic("cannot use continuations inside hooks")
	}
	l.checkElementCount(argCount + 1)
	if apiCheck && l.shouldYield {
		panic("cannot do calls on non-normal thread")
	}
	l.checkResults(argCount, resultCount)
	if errorFunction != 0 {
		apiCheckStackIndex(errorFunction, l.indexToValue(errorFunction))
		errorFunction = l.AbsIndex(errorFunction)
	}

	f := l.top - (argCount + 1)

	if continuation == nil || l.nonYieldableCallCount > 0 {
		err = l.protectedCall(func() { l.call(f, resultCount, false) }, f, errorFunction)
	} else {
		c := l.callInfo
		c.continuation, c.context, c.extra, c.oldAllowHook, c.oldErrorFunction = continuation, context, f, l.allowHook, l.errorFunction
		l.errorFunction = errorFunction
		l.callInfo.setCallStatus(callStatusYieldableProtected)
		l.call(f, resultCount, true)
		l.callInfo.clearCallStatus(callStatusYieldableProtected)
		l.errorFunction = c.oldErrorFunction
	}
	l.adjustResults(resultCount)
	return
}

// Load loads a Lua chunk, without running it. If there are no errors, it
// pushes the compiled chunk as a Lua function on top of the stack.
// Otherwise, it pushes an error message.
//
// http://www.lua.org/manual/5.2/manual.html#lua_load
func (l *State) Load(r io.Reader, chunkName string, mode string) error {
	if chunkName == "" {
		chunkName = "?"
	}

	if err := protectedParser(l, r, chunkName, mode); err != nil {
		return err
	}

	if f := l.stack[l.top-1].(*luaClosure); f.upValueCount() == 1 {
		f.setUpValue(0, l.global.registry.atInt(RegistryIndexGlobals))
	}
	return nil
}

// Dump dumps a function as a binary chunk. It receives a Lua function on
// the top of the stack and produces a binary chunk that, if loaded again,
// results in a function equivalent to the one dumped.
//
// http://www.lua.org/manual/5.3/manual.html#lua_dump
func (l *State) Dump(w io.Writer) error {
	l.checkElementCount(1)
	if f, ok := l.stack[l.top-1].(*luaClosure); ok {
		return l.dump(f.prototype, w)
	}
	panic("closure expected")
}

// NewState creates a new thread running in a new, independent state.
//
// http://www.lua.org/manual/5.2/manual.html#lua_newstate
func NewState() *State {
	v := float64(VersionNumber)
	l := &State{allowHook: true, error: nil, nonYieldableCallCount: 1}
	g := &globalState{mainThread: l, registry: newTable(), version: &v, memoryErrorMessage: "not enough memory"}
	l.global = g
	l.initializeStack()
	g.registry.putAtInt(RegistryIndexMainThread, l)
	g.registry.putAtInt(RegistryIndexGlobals, newTable())
	copy(g.tagMethodNames[:], eventNames)
	return l
}

func apiCheckStackIndex(index int, v value) {
	if apiCheck && (v == none || isPseudoIndex(index)) {
		panic(fmt.Sprintf("index %d not in the stack", index))
	}
}

// SetField does the equivalent of table[key]=v where table is the value at
// index and v is the value on top of the stack.
//
// This function pops the value from the stack. As in Lua, this function may
// trigger a metamethod for the __newindex event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setfield
func (l *State) SetField(index int, key string) {
	l.checkElementCount(1)
	t := l.indexToValue(index)
	l.push(key)
	l.setTableAt(t, key, l.stack[l.top-2])
	l.top -= 2
}

var none value = &struct{}{}

func (l *State) indexToValue(index int) value {
	switch {
	case index > 0:
		// TODO apiCheck(index <= callInfo.top_-(callInfo.function+1), "unacceptable index")
		// if i := callInfo.function + index; i < l.top {
		// 	return l.stack[i]
		// }
		// return none
		if l.callInfo.function+index >= l.top {
			return none
		}
		return l.stack[l.callInfo.function:l.top][index]
	case index > RegistryIndex: // negative index
		// TODO apiCheck(index != 0 && -index <= l.top-(callInfo.function+1), "invalid index")
		return l.stack[l.top+index]
	case index == RegistryIndex:
		return l.global.registry
	default: // upvalues
		i := RegistryIndex - index
		return l.stack[l.callInfo.function].(*goClosure).upValues[i-1]
		// if closure := l.stack[callInfo.function].(*goClosure); i <= len(closure.upValues) {
		// 	return closure.upValues[i-1]
		// }
		// return none
	}
}

func (l *State) setIndexToValue(index int, v value) {
	switch {
	case index > 0:
		l.stack[l.callInfo.function:l.top][index] = v
		// if i := callInfo.function + index; i < l.top {
		// 	l.stack[i] = v
		// } else {
		// 	panic("unacceptable index")
		// }
	case index > RegistryIndex: // negative index
		l.stack[l.top+index] = v
	case index == RegistryIndex:
		l.global.registry = v.(*table)
	default: // upvalues
		i := RegistryIndex - index
		l.stack[l.callInfo.function].(*goClosure).upValues[i-1] = v
	}
}

// AbsIndex converts the acceptable index index to an absolute index (that
// is, one that does not depend on the stack top).
//
// http://www.lua.org/manual/5.2/manual.html#lua_absindex
func (l *State) AbsIndex(index int) int {
	if index > 0 || isPseudoIndex(index) {
		return index
	}
	return l.top - l.callInfo.function + index
}

// SetTop accepts any index, or 0, and sets the stack top to index. If the
// new top is larger than the old one, then the new elements are filled with
// nil. If index is 0, then all stack elements are removed.
//
// If index is negative, the stack will be decremented by that much. If
// the decrement is larger than the stack, SetTop will panic().
//
// http://www.lua.org/manual/5.2/manual.html#lua_settop
func (l *State) SetTop(index int) {
	f := l.callInfo.function
	if index >= 0 {
		if apiCheck && index > l.stackLast-(f+1) {
			panic("new top too large")
		}
		i := l.top
		for l.top = f + 1 + index; i < l.top; i++ {
			l.stack[i] = nil
		}
	} else {
		if apiCheck && -(index+1) > l.top-(f+1) {
			panic("invalid new top")
		}
		l.top += index + 1 // 'subtract' index (index is negative)
	}
}

// Remove the element at the given valid index, shifting down the elements
// above index to fill the gap. This function cannot be called with a
// pseudo-index, because a pseudo-index is not an actual stack position.
//
// http://www.lua.org/manual/5.2/manual.html#lua_remove
func (l *State) Remove(index int) {
	apiCheckStackIndex(index, l.indexToValue(index))
	i := l.callInfo.function + l.AbsIndex(index)
	copy(l.stack[i:l.top-1], l.stack[i+1:l.top])
	l.top--
}

// Insert moves the top element into the given valid index, shifting up the
// elements above this index to open space.  This function cannot be called
// with a pseudo-index, because a pseudo-index is not an actual stack position.
//
// http://www.lua.org/manual/5.2/manual.html#lua_insert
func (l *State) Insert(index int) {
	apiCheckStackIndex(index, l.indexToValue(index))
	i := l.callInfo.function + l.AbsIndex(index)
	copy(l.stack[i+1:l.top+1], l.stack[i:l.top])
	l.stack[i] = l.stack[l.top]
}

func (l *State) move(dest int, src value) { l.setIndexToValue(dest, src) }

// Replace moves the top element into the given valid index without shifting
// any element (therefore replacing the value at the given index), and then
// pops the top element.
//
// http://www.lua.org/manual/5.2/manual.html#lua_replace
func (l *State) Replace(index int) {
	l.checkElementCount(1)
	l.move(index, l.stack[l.top-1])
	l.top--
}

// CheckStack ensures that there are at least size free stack slots in the
// stack. This call will not panic(), unlike the other Check*() functions.
//
// http://www.lua.org/manual/5.2/manual.html#lua_checkstack
func (l *State) CheckStack(size int) bool {
	callInfo := l.callInfo
	ok := l.stackLast-l.top > size
	if !ok && l.top+extraStack <= maxStack-size {
		ok = l.protect(func() { l.growStack(size) }) == nil
	}
	if ok && callInfo.top < l.top+size {
		callInfo.setTop(l.top + size)
	}
	return ok
}

// AtPanic sets a new panic function and returns the old one.
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
	case *goFunction:
		return TypeFunction
	case *userData:
		return TypeUserData
	case *State:
		return TypeThread
	case *luaClosure:
		return TypeFunction
	case *goClosure:
		return TypeFunction
	}
	return TypeNone
}

// TypeOf returns the type of the value at index, or TypeNone for a
// non-valid (but acceptable) index.
//
// http://www.lua.org/manual/5.2/manual.html#lua_type
func (l *State) TypeOf(index int) Type {
	return l.valueToType(l.indexToValue(index))
}

// IsGoFunction verifies that the value at index is a Go function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_iscfunction
func (l *State) IsGoFunction(index int) bool {
	if _, ok := l.indexToValue(index).(*goFunction); ok {
		return true
	}
	_, ok := l.indexToValue(index).(*goClosure)
	return ok
}

// IsNumber verifies that the value at index is a number.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnumber
func (l *State) IsNumber(index int) bool {
	_, ok := l.toNumber(l.indexToValue(index))
	return ok
}

// IsString verifies that the value at index is a string, or a number (which
// is always convertible to a string).
//
// http://www.lua.org/manual/5.2/manual.html#lua_isstring
func (l *State) IsString(index int) bool {
	if _, ok := l.indexToValue(index).(string); ok {
		return true
	}
	_, ok := l.indexToValue(index).(float64)
	return ok
}

// IsUserData verifies that the value at index is a userdata.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isuserdata
func (l *State) IsUserData(index int) bool {
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
func (l *State) Arith(op Operator) {
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

// RawEqual verifies that the values at index1 and index2 are primitively
// equal (that is, without calling their metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawequal
func (l *State) RawEqual(index1, index2 int) bool {
	if o1, o2 := l.indexToValue(index1), l.indexToValue(index2); o1 != nil && o2 != nil {
		return o1 == o2
	}
	return false
}

// Compare compares two values.
//
// http://www.lua.org/manual/5.2/manual.html#lua_compare
func (l *State) Compare(index1, index2 int, op ComparisonOperator) bool {
	if o1, o2 := l.indexToValue(index1), l.indexToValue(index2); o1 != nil && o2 != nil {
		switch op {
		case OpEq:
			return l.equalObjects(o1, o2)
		case OpLT:
			return l.lessThan(o1, o2)
		case OpLE:
			return l.lessOrEqual(o1, o2)
		default:
			panic("invalid option")
		}
	}
	return false
}

// ToInteger converts the Lua value at index into a signed integer. The Lua
// value must be a number, or a string convertible to a number.
//
// If the number is not an integer, it is truncated in some non-specified way.
//
// If the operation failed, the second return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tointegerx
func (l *State) ToInteger(index int) (int, bool) {
	if n, ok := l.toNumber(l.indexToValue(index)); ok {
		return int(n), true
	}
	return 0, false
}

// ToUnsigned converts the Lua value at index to a Go uint. The Lua value
// must be a number or a string convertible to a number.
//
// If the number is not an unsigned integer, it is truncated in some
// non-specified way.  If the number is outside the range of uint, it is
// normalized to the remainder of its division by one more than the maximum
// representable value.
//
// If the operation failed, the second return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tounsignedx
func (l *State) ToUnsigned(index int) (uint, bool) {
	if n, ok := l.toNumber(l.indexToValue(index)); ok {
		const supUnsigned = float64(^uint32(0)) + 1
		return uint(n - math.Floor(n/supUnsigned)*supUnsigned), true
	}
	return 0, false
}

// ToString  converts the Lua value at index to a Go string.  The Lua value
// must also be a string or a number; otherwise the function returns
// false for its second return value.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tolstring
func (l *State) ToString(index int) (s string, ok bool) {
	if s, ok = toString(l.indexToValue(index)); ok { // Bug compatibility: replace a number with its string representation.
		l.setIndexToValue(index, s)
	}
	return
}

// RawLength returns the length of the value at index.  For strings, this is
// the length.  For tables, this is the result of the # operator with no
// metamethods.  For userdata, this is the size of the block of memory
// allocated for the userdata (not implemented yet). For other values, it is 0.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawlen
func (l *State) RawLength(index int) int {
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

// ToGoFunction converts a value at index into a Go function.  That value
// must be a Go function, otherwise it returns nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tocfunction
func (l *State) ToGoFunction(index int) Function {
	switch v := l.indexToValue(index).(type) {
	case *goFunction:
		return v.Function
	case *goClosure:
		return v.function
	}
	return nil
}

// ToUserData returns an interface{} of the userdata of the value at index.
// Otherwise, it returns nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_touserdata
func (l *State) ToUserData(index int) interface{} {
	if d, ok := l.indexToValue(index).(*userData); ok {
		return d.data
	}
	return nil
}

// ToThread converts the value at index to a Lua thread (a State). This
// value must be a thread, otherwise the return value will be nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tothread
func (l *State) ToThread(index int) *State {
	if t, ok := l.indexToValue(index).(*State); ok {
		return t
	}
	return nil
}

// ToValue convertes the value at index into a generic Go interface{}.  The
// value can be a userdata, a table, a thread, a function, or Go string, bool
// or float64 types. Otherwise, the function returns nil.
//
// Different objects will give different values.  There is no way to convert
// the value back into its original value.
//
// Typically, this function is used only for debug information.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tovalue
func (l *State) ToValue(index int) interface{} {
	v := l.indexToValue(index)
	switch v := v.(type) {
	case string, float64, bool, *table, *luaClosure, *goClosure, *goFunction, *State:
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
func (l *State) PushString(s string) string { // TODO is it useful to return the argument?
	l.apiPush(s)
	return s
}

// PushFString pushes onto the stack a formatted string and returns that
// string.  It is similar to fmt.Sprintf, but has some differences: the
// conversion specifiers are quite restricted.  There are no flags, widths,
// or precisions.  The conversion specifiers can only be %% (inserts a %
// in the string), %s, %f (a Lua number), %p (a pointer as a hexadecimal
// numeral), %d and %c (an integer as a byte).
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushfstring
func (l *State) PushFString(format string, args ...interface{}) string {
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
			i++
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
// function whenever it is called.  To associate values with a Go function,
// first these values should be pushed onto the stack (when there are multiple
// values, the first value is pushed first).  Then PushGoClosure is called to
// create and push the Go function onto the stack, with the argument upValueCount
// telling how many values should be associated with the function.  Calling
// PushGoClosure also pops these values from the stack.
//
// When upValueCount is 0, this function creates a light Go function, which is just a
// Go function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushcclosure
func (l *State) PushGoClosure(function Function, upValueCount uint8) {
	if upValueCount == 0 {
		l.apiPush(&goFunction{function})
	} else {
		n := int(upValueCount)

		l.checkElementCount(n)
		cl := &goClosure{function: function, upValues: make([]value, upValueCount)}
		l.top -= n
		copy(cl.upValues, l.stack[l.top:l.top+n])
		l.apiPush(cl)
	}
}

// PushThread pushes the thread l onto the stack.  It returns true if l is
// the main thread of its state.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushthread
func (l *State) PushThread() bool {
	l.apiPush(l)
	return l.global.mainThread == l
}

// Global pushes onto the stack the value of the global name.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getglobal
func (l *State) Global(name string) {
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.stack[l.top-1] = l.tableAt(g, l.stack[l.top-1])
}

// Field pushes onto the stack the value table[name], where table is the
// table on the stack at the given index. This call may trigger a
// metamethod for the __index event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getfield
func (l *State) Field(index int, name string) {
	t := l.indexToValue(index)
	l.apiPush(name)
	l.stack[l.top-1] = l.tableAt(t, l.stack[l.top-1])
}

// RawGet is similar to GetTable, but does a raw access (without metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawget
func (l *State) RawGet(index int) {
	t := l.indexToValue(index).(*table)
	l.stack[l.top-1] = t.at(l.stack[l.top-1])
}

// RawGetInt pushes onto the stack the value table[key] where table is the
// value at index on the stack. The access is raw, as it doesn't invoke
// metamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawgeti
func (l *State) RawGetInt(index, key int) {
	t := l.indexToValue(index).(*table)
	l.apiPush(t.atInt(key))
}

// RawGetValue pushes onto the stack value table[p] where table is the
// value at index on the stack, and p is a light userdata.  The access is
// raw, as it doesn't invoke metamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawgetp
func (l *State) RawGetValue(index int, p interface{}) {
	t := l.indexToValue(index).(*table)
	l.apiPush(t.at(p))
}

// CreateTable creates a new empty table and pushes it onto the stack.
// arrayCount is a hint for how many elements the table will have as a
// sequence; recordCount is a hint for how many other elements the table
// will have.  Lua may use these hints to preallocate memory for the the new
// table.  This pre-allocation is useful for performance when you know in
// advance how many elements the table will have.  Otherwise, you can use the
// function NewTable.
//
// http://www.lua.org/manual/5.2/manual.html#lua_createtable
func (l *State) CreateTable(arrayCount, recordCount int) {
	l.apiPush(newTableWithSize(arrayCount, recordCount))
}

// MetaTable pushes onto the stack the metatable of the value at index.  If
// the value at index does not have a metatable, the function returns
// false and nothing is put onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getmetatable
func (l *State) MetaTable(index int) bool {
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
// at index.  This value must be a table or nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_getuservalue
func (l *State) UserValue(index int) {
	d := l.indexToValue(index).(*userData)
	if d.env == nil {
		l.apiPush(nil)
	} else {
		l.apiPush(d.env)
	}
}

// SetGlobal pops a value from the stack and sets it as the new value of
// global name.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setglobal
func (l *State) SetGlobal(name string) {
	l.checkElementCount(1)
	g := l.global.registry.atInt(RegistryIndexGlobals)
	l.push(name)
	l.setTableAt(g, l.stack[l.top-1], l.stack[l.top-2])
	l.top -= 2 // pop value and key
}

// SetTable does the equivalent of table[key]=v, where table is the value
// at index, v is the value at the top of the stack and key is the value
// just below the top.
//
// The function pops both the key and the value from the stack.  As in Lua,
// this function may trigger a metamethod for the __newindex event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_settable
func (l *State) SetTable(index int) {
	l.checkElementCount(2)
	l.setTableAt(l.indexToValue(index), l.stack[l.top-2], l.stack[l.top-1])
	l.top -= 2
}

// RawSet is similar to SetTable, but does a raw assignment (without
// metamethods).
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawset
func (l *State) RawSet(index int) {
	l.checkElementCount(2)
	t := l.indexToValue(index).(*table)
	t.put(l, l.stack[l.top-2], l.stack[l.top-1])
	t.invalidateTagMethodCache()
	l.top -= 2
}

// RawSetInt does the equivalent of table[n]=v where table is the table at
// index and v is the value at the top of the stack.
//
// This function pops the value from the stack.  The assignment is raw; it
// doesn't invoke metamethods.
//
// http://www.lua.org/manual/5.2/manual.html#lua_rawseti
func (l *State) RawSetInt(index, key int) {
	l.checkElementCount(1)
	t := l.indexToValue(index).(*table)
	t.putAtInt(key, l.stack[l.top-1])
	l.top--
}

// SetUserValue pops a table or nil from the stack and sets it as the new
// value associated to the userdata at index.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setuservalue
func (l *State) SetUserValue(index int) {
	l.checkElementCount(1)
	d := l.indexToValue(index).(*userData)
	if l.stack[l.top-1] == nil {
		d.env = nil
	} else {
		t := l.stack[l.top-1].(*table)
		d.env = t
	}
	l.top--
}

// SetMetaTable pops a table from the stack and sets it as the new metatable
// for the value at index.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setmetatable
func (l *State) SetMetaTable(index int) {
	l.checkElementCount(1)
	mt, ok := l.stack[l.top-1].(*table)
	if apiCheck && !ok && l.stack[l.top-1] != nil {
		panic("table expected")
	}
	switch v := l.indexToValue(index).(type) {
	case *table:
		v.metaTable = mt
	case *userData:
		v.metaTable = mt
	default:
		l.global.metaTables[l.TypeOf(index)] = mt
	}
	l.top--
}

// Error generates a Lua error.  The error message must be on the stack top.
// The error can be any of any Lua type. This function will panic().
//
// http://www.lua.org/manual/5.2/manual.html#lua_error
func (l *State) Error() {
	l.checkElementCount(1)
	l.errorMessage()
}

// Next pops a key from the stack and pushes a key-value pair from the table
// at index, while the table has next elements.  If there are no more
// elements, nothing is pushed on the stack and Next returns false.
//
// A typical traversal looks like this:
//
//  // Table is on top of the stack (index -1).
//  l.PushNil() // Add nil entry on stack (need 2 free slots).
//  for l.Next(-2) {
//  	key := lua.CheckString(l, -2)
//  	val := lua.CheckString(l, -1)
//  	l.Pop(1) // Remove val, but need key for the next iter.
//  }
//
// http://www.lua.org/manual/5.2/manual.html#lua_next
func (l *State) Next(index int) bool {
	t := l.indexToValue(index).(*table)
	if l.next(t, l.top-1) {
		l.apiIncrementTop()
		return true
	}
	// no more elements
	l.top-- // remove key
	return false
}

// Concat concatenates the n values at the top of the stack, pops them, and
// leaves the result at the top. If n is 1, the result is the single value
// on the stack (that is, the function does nothing); if n is 0, the result
// is the empty string. Concatenation is performed following the usual
// semantic of Lua.
//
// http://www.lua.org/manual/5.2/manual.html#lua_concat
func (l *State) Concat(n int) {
	l.checkElementCount(n)
	if n >= 2 {
		l.concat(n)
	} else if n == 0 { // push empty string
		l.apiPush("")
	} // else n == 1; nothing to do
}

// Register sets the Go function f as the new value of global name. If
// name was already defined, it is overwritten.
//
// http://www.lua.org/manual/5.2/manual.html#lua_register
func (l *State) Register(name string, f Function) {
	l.PushGoFunction(f)
	l.SetGlobal(name)
}

func (l *State) setErrorObject(err error, oldTop int) {
	switch err {
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
	err := l.protect(f)
	if err != nil {
		l.close(oldTop)
		l.setErrorObject(err, oldTop)
		l.callInfo, l.allowHook, l.nonYieldableCallCount = callInfo, allowHook, nonYieldableCallCount
		// TODO l.shrinkStack()
	}
	l.errorFunction = errorFunction
	return err
}

// UpValue returns the name of the upvalue at index away from function,
// where index cannot be greater than the number of upvalues.
//
// Returns an empty string and false if the index is greater than the number
// of upvalues.
func UpValue(l *State, function, index int) (name string, ok bool) {
	if c, isClosure := l.indexToValue(function).(closure); isClosure {
		if ok = 1 <= index && index <= c.upValueCount(); ok {
			if c, isLua := c.(*luaClosure); isLua {
				name = c.prototype.upValues[index-1].name
			}
			l.apiPush(c.upValue(index - 1))
		}
	}
	return
}

// SetUpValue sets the value of a closure's upvalue. It assigns the value at
// the top of the stack to the upvalue and returns its name.  It also pops a
// value from the stack. function and index are as in UpValue.
//
// Returns an empty string and false if the index is greater than the number
// of upvalues.
//
// http://www.lua.org/manual/5.2/manual.html#lua_setupvalue
func SetUpValue(l *State, function, index int) (name string, ok bool) {
	if c, isClosure := l.indexToValue(function).(closure); isClosure {
		if ok = 1 <= index && index <= c.upValueCount(); ok {
			if c, isLua := c.(*luaClosure); isLua {
				name = c.prototype.upValues[index-1].name
			}
			l.top--
			c.setUpValue(index-1, l.stack[l.top])
		}
	}
	return
}

func (l *State) upValue(f, n int) **upValue {
	return &l.indexToValue(f).(*luaClosure).upValues[n-1]
}

// UpValueId returns a unique identifier for the upvalue numbered n from the
// closure at index f. Parameters f and n are as in UpValue (but n cannot be
// greater than the number of upvalues).
//
// These unique identifiers allow a program to check whether different
// closures share upvalues. Lua closures that share an upvalue (that is, that
// access a same external local variable) will return identical ids for those
// upvalue indices.
func UpValueId(l *State, f, n int) interface{} {
	switch fun := l.indexToValue(f).(type) {
	case *luaClosure:
		return *l.upValue(f, n)
	case *goClosure:
		return &fun.upValues[n-1]
	}
	panic("closure expected")
}

// UpValueJoin makes the n1-th upvalue of the Lua closure at index f1 refer to
// the n2-th upvalue of the Lua closure at index f2.
func UpValueJoin(l *State, f1, n1, f2, n2 int) {
	u1 := l.upValue(f1, n1)
	u2 := l.upValue(f2, n2)
	*u1 = *u2
}

// Call calls a function. To do so, use the following protocol: first, the
// function to be called is pushed onto the stack; then, the arguments to the
// function are pushed in direct order - that is, the first argument is pushed
// first. Finally, call Call. argCount is the number of arguments that you
// pushed onto the stack. All arguments and the function value are popped
// from the stack when the function is called.
//
// The results are pushed onto the stack when the function returns. The
// number of results is adjusted to resultCount, unless resultCount is
// MultipleReturns. In this case, all results from the function are pushed.
// Lua takes care that the returned values fit into the stack space. The
// function results are pushed onto the stack in direct order (the first
// result is pushed first), so that after the call the last result is on the
// top of the stack.
//
// Any error inside the called function provokes a call to panic().
//
// The following example shows how the host program can do the equivalent to
// this Lua code:
//
//		a = f("how", t.x, 14)
//
// Here it is in Go:
//
//		l.Global("f")       // Function to be called.
//		l.PushString("how") // 1st argument.
//		l.Global("t")       // Table to be indexed.
//		l.Field(-1, "x")    // Push result of t.x (2nd arg).
//		l.Remove(-2)        // Remove t from the stack.
//		l.PushInteger(14)   // 3rd argument.
//		l.Call(3, 1)        // Call f with 3 arguments and 1 result.
//		l.SetGlobal("a")    // Set global a.
//
// Note that the code above is "balanced": at its end, the stack is back to
// its original configuration. This is considered good programming practice.
//
// http://www.lua.org/manual/5.2/manual.html#lua_call
func (l *State) Call(argCount, resultCount int) {
	l.CallWithContinuation(argCount, resultCount, 0, nil)
}

// Top returns the index of the top element in the stack. Because Lua indices
// start at 1, this result is equal to the number of elements in the stack
// (hence 0 means an empty stack).
//
// http://www.lua.org/manual/5.2/manual.html#lua_gettop
func (l *State) Top() int { return l.top - (l.callInfo.function + 1) }

// Copy moves the element at the index from into the valid index to
// without shifting any element (therefore replacing the value at that
// position).
//
// http://www.lua.org/manual/5.2/manual.html#lua_copy
func (l *State) Copy(from, to int) { l.move(to, l.indexToValue(from)) }

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

func apiCheckStackSpace(l *State, n int) { l.assert(n < l.top-l.callInfo.function) }

// String returns the name of Type t.
//
// http://www.lua.org/manual/5.2/manual.html#lua_typename
func (t Type) String() string { return typeNames[t+1] }

// ToNumber converts the Lua value at index to the Go type for a Lua number
// (float64). The Lua value must be a number or a string convertible to a
// number.
//
// If the operation failed, the second return value will be false.
//
// http://www.lua.org/manual/5.2/manual.html#lua_tonumberx
func (l *State) ToNumber(index int) (float64, bool) { return l.toNumber(l.indexToValue(index)) }

// ToBoolean converts the Lua value at index to a Go boolean. Like all
// tests in Lua, the only false values are false booleans and nil.
// Otherwise, all other Lua values evaluate to true.
//
// To accept only actual boolean values, use the test IsBoolean.
//
// http://www.lua.org/manual/5.2/manual.html#lua_toboolean
func (l *State) ToBoolean(index int) bool { return !isFalse(l.indexToValue(index)) }

// Table pushes onto the stack the value table[top], where table is the
// value at index, and top is the value at the top of the stack. This
// function pops the key from the stack, putting the resulting value in its
// place.  As in Lua, this function may trigger a metamethod for the __index
// event.
//
// http://www.lua.org/manual/5.2/manual.html#lua_gettable
func (l *State) Table(index int) {
	l.stack[l.top-1] = l.tableAt(l.indexToValue(index), l.stack[l.top-1])
}

// PushValue pushes a copy of the element at index onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushvalue
func (l *State) PushValue(index int) { l.apiPush(l.indexToValue(index)) }

// PushNil pushes a nil value onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushnil
func (l *State) PushNil() { l.apiPush(nil) }

// PushNumber pushes a number onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushnumber
func (l *State) PushNumber(n float64) { l.apiPush(n) }

// PushInteger pushes n onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushinteger
func (l *State) PushInteger(n int) { l.apiPush(float64(n)) }

// PushUnsigned pushes n onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushunsigned
func (l *State) PushUnsigned(n uint) { l.apiPush(float64(n)) }

// PushBoolean pushes a boolean value with value b onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushboolean
func (l *State) PushBoolean(b bool) { l.apiPush(b) }

// PushLightUserData pushes a light user data onto the stack. Userdata
// represents Go values in Lua. A light userdata is an interface{}. Its
// equality matches the Go rules (http://golang.org/ref/spec#Comparison_operators).
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushlightuserdata
func (l *State) PushLightUserData(d interface{}) { l.apiPush(d) }

// PushUserData is similar to PushLightUserData, but pushes a full userdata
// onto the stack.
func (l *State) PushUserData(d interface{}) { l.apiPush(&userData{data: d}) }

// Length of the value at index; it is equivalent to the # operator in
// Lua. The result is pushed on the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_len
func (l *State) Length(index int) { l.apiPush(l.objectLength(l.indexToValue(index))) }

// Pop pops n elements from the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pop
func (l *State) Pop(n int) { l.SetTop(-n - 1) }

// NewTable creates a new empty table and pushes it onto the stack. It is
// equivalent to l.CreateTable(0, 0).
//
// http://www.lua.org/manual/5.2/manual.html#lua_newtable
func (l *State) NewTable() { l.CreateTable(0, 0) }

// PushGoFunction pushes a Function implemented in Go onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushcfunction
func (l *State) PushGoFunction(f Function) { l.PushGoClosure(f, 0) }

// IsFunction verifies that the value at index is a function, either Go or
// Lua function.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isfunction
func (l *State) IsFunction(index int) bool { return l.TypeOf(index) == TypeFunction }

// IsTable verifies that the value at index is a table.
//
// http://www.lua.org/manual/5.2/manual.html#lua_istable
func (l *State) IsTable(index int) bool { return l.TypeOf(index) == TypeTable }

// IsLightUserData verifies that the value at index is a light userdata.
//
// http://www.lua.org/manual/5.2/manual.html#lua_islightuserdata
func (l *State) IsLightUserData(index int) bool { return l.TypeOf(index) == TypeLightUserData }

// IsNil verifies that the value at index is nil.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnil
func (l *State) IsNil(index int) bool { return l.TypeOf(index) == TypeNil }

// IsBoolean verifies that the value at index is a boolean.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isboolean
func (l *State) IsBoolean(index int) bool { return l.TypeOf(index) == TypeBoolean }

// IsThread verifies that the value at index is a thread.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isthread
func (l *State) IsThread(index int) bool { return l.TypeOf(index) == TypeThread }

// IsNone verifies that the value at index is not valid.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnone
func (l *State) IsNone(index int) bool { return l.TypeOf(index) == TypeNone }

// IsNoneOrNil verifies that the value at index is either nil or invalid.
//
// http://www.lua.org/manual/5.2/manual.html#lua_isnonornil.
func (l *State) IsNoneOrNil(index int) bool { return l.TypeOf(index) <= TypeNil }

// PushGlobalTable pushes the global environment onto the stack.
//
// http://www.lua.org/manual/5.2/manual.html#lua_pushglobaltable
func (l *State) PushGlobalTable() { l.RawGetInt(RegistryIndex, RegistryIndexGlobals) }
