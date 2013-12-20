package lua

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

const (
	Ok = iota
	Yield
	RuntimeError
	SyntaxError
	MemoryError
	GCError
	Error
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
	RegistryIndexMainThread = iota
	RegistryIndexGlobals
)

const (
	Signature     = "\033Lua" // mark for precompiled code ('<esc>Lua')
	VersionMajor  = 5
	VersionMinor  = 2
	VersionNumber = 502
	MinStack      = 20 // minimum Lua stack available to a Go function
)

type Debug struct {
	Event                                     int
	Name                                      string
	NameKind                                  string // "global", "local", "field", "method"
	What                                      string // "Lua", "Go", "main", "tail"
	Source                                    string
	CurrentLine, LineDefined, LastLineDefined int
	UpValueCount, ParameterCount              byte
	IsVarArg, IsTailCall                      bool
	callInfo                                  callInfo // active function
}

type Hook func(state State, activationRecord *Debug)
type Function func(state State) int

type State interface {
	// // Miscellaneous functions
	// Error()
	// Next(index int) int
	// Concat(n int)
	// Length(index int)
	// //AllocateFunction(userData interface{}) ...
	// //SetAllocateFunction(..., userData interface{})
	// //NewUserData(size int) interface{}
	// UpValue(f, n int) string
	// SetUpValue(f, n int) string
	// UpValueId(f, n int) interface{}
	// UpValueJoin(f1, n1, f2, n2 int)
	ApiCheckStackSpace(n int)
	Version() *float64
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
	status                byte
	top                   stackIndex // first free slot in the stack
	global                *globalState
	callInfo              callInfo   // call info for current function
	oldPC                 pc         // last pC traced
	stackLast             stackIndex // last free slot in the stack
	stack                 []value
	nonYieldableCallCount uint16
	nestedGoCallCount     uint16
	hookMask              byte
	allowHook             bool
	baseHookCount         int
	hookCount             int
	hooker                Hook
	upValues              *openUpValue
	// errorJmp *longjmp // current error recover point
	// errorFunc stackIndex // current error handling function (stack index)
	baseCallInfo luaCallInfo // callInfo for first level (go calling lua)
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
		// TODO TypeFunction
		// TODO TypeUserData
		// TODO TypeThread
	default:
		return nil
	}
	return g.metaTables[t]
}

func (l *state) ApiCheckStackSpace(n int) {
	l.assert(n < int(l.top-l.callInfo.function()))
}

func (l *state) adjustResults(resultCount int) {
	if resultCount == MultipleReturns && l.callInfo.top() < l.top {
		l.callInfo.setTop(l.top)
		// TODO adjust l.callInfo.frame?
	}
}

func (l *state) Call(argCount, resultCount, context int, k interface{}) { // TODO
	// check k==nil || !isLua(L.ci) "cannot use continuations inside hooks"
	// check element count argCount+1
	// check l.status==OK "cannot do calls on non-normal thread"
	// checkresults argCount, resultCount
	f := l.top - stackIndex(argCount+1)
	if k != nil && l.nonYieldableCallCount == 0 { // need to prepare continuation?
		panic("continuations not yet supported")
		l.call(f, int16(resultCount), true) // just do the call
	} else { // no continuation or not yieldable
		l.call(f, int16(resultCount), false) // just do the call
	}
	// adjustresults resultCount
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
	for i := range g.tagMethodNames {
		g.tagMethodNames[i] = eventNames[i]
	}
	return l
}
