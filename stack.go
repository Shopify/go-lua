package lua

func (l *State) push(v value) {
	l.stack[l.top] = v
	l.top++
}

func (l *State) pop() value {
	l.top--
	return l.stack[l.top]
}

type upValue struct {
	home interface{}
}

type closure interface {
	upValue(i int) value
	setUpValue(i int, v value)
	upValueCount() int
}

type luaClosure struct {
	prototype *prototype
	upValues  []*upValue
}

type goClosure struct {
	function Function
	upValues []value
}

func (c *luaClosure) upValue(i int) value {
	return c.upValues[i].value()
}

func (c *luaClosure) setUpValue(i int, v value) {
	c.upValues[i].setValue(v)
}

func (c *luaClosure) upValueCount() int {
	return len(c.upValues)
}

func (c *goClosure) upValue(i int) value {
	return c.upValues[i]
}

func (c *goClosure) setUpValue(i int, v value) {
	c.upValues[i] = v
}

func (c *goClosure) upValueCount() int {
	return len(c.upValues)
}

func (l *State) newUpValue() *upValue {
	return &upValue{home: nil}
}

func (uv *upValue) setValue(v value) {
	if home, ok := uv.home.(stackLocation); ok {
		home.state.stack[home.index] = v
	} else {
		uv.home = v
	}
}

func (uv *upValue) value() value {
	if home, ok := uv.home.(stackLocation); ok {
		return home.state.stack[home.index]
	}
	return uv.home
}

func (uv *upValue) close() {
	if home, ok := uv.home.(stackLocation); ok {
		uv.home = home.state.stack[home.index]
	} else {
		panic("attempt to close already-closed up value")
	}
}

func (uv *upValue) isInStackAt(level int) bool {
	if home, ok := uv.home.(stackLocation); ok {
		return home.index == level
	}
	return false
}

func (uv *upValue) isInStackBelow(level int) bool {
	if home, ok := uv.home.(stackLocation); ok {
		return home.index < level
	}
	return false
}

type openUpValue struct {
	upValue *upValue
	next    *openUpValue
}

func (l *State) newUpValueAt(level int) *upValue {
	uv := &upValue{home: stackLocation{state: l, index: level}}
	l.upValues = &openUpValue{upValue: uv, next: l.upValues}
	return uv
}

func (l *State) close(level int) {
	for e := l.upValues; e != nil; e = e.next {
		if e.upValue.isInStackBelow(level) {
			l.upValues = e
			return
		}
		e.upValue.close()
	}
	l.upValues = nil
}

// information about a call
type callInfo interface {
	next() callInfo
	previous() callInfo
	push(callInfo) callInfo
	function() int
	top() int
	setTop(int)
	isLua() bool
	callStatus() callStatus
	setCallStatus(callStatus)
	clearCallStatus(callStatus)
	isCallStatus(callStatus) bool
	resultCount() int
}

type commonCallInfo struct {
	function_        int
	top_             int
	previous_, next_ callInfo
	resultCount_     int
	callStatus_      callStatus
}

func (ci *commonCallInfo) top() int {
	return ci.top_
}

func (ci *commonCallInfo) setTop(top int) {
	ci.top_ = top
}

func (ci *commonCallInfo) next() callInfo {
	return ci.next_
}

func (ci *commonCallInfo) previous() callInfo {
	return ci.previous_
}

func (ci *commonCallInfo) push(nci callInfo) callInfo {
	ci.next_ = nci
	return nci
}

func (ci *commonCallInfo) function() int {
	return ci.function_
}

func (ci *commonCallInfo) resultCount() int {
	return ci.resultCount_
}

func (ci *commonCallInfo) callStatus() callStatus {
	return ci.callStatus_
}

func (ci *commonCallInfo) setCallStatus(flag callStatus) {
	ci.callStatus_ |= flag
}

func (ci *commonCallInfo) clearCallStatus(flag callStatus) {
	ci.callStatus_ &^= flag
}

func (ci *commonCallInfo) isCallStatus(flag callStatus) bool {
	return ci.callStatus_&flag != 0
}

func (ci *commonCallInfo) initialize(l *State, function, top, resultCount int, callStatus callStatus) {
	ci.function_ = function
	ci.top_ = top
	l.assert(ci.top() <= l.stackLast)
	ci.resultCount_ = resultCount
	ci.callStatus_ = callStatus
}

type luaCallInfo struct {
	commonCallInfo
	frame   []value
	savedPC pc
	code    []instruction
}

func (ci *luaCallInfo) isLua() bool {
	return true
}

func (ci *luaCallInfo) setTop(top int) {
	diff := top - ci.top()
	ci.frame = ci.frame[:len(ci.frame)+diff]
	ci.commonCallInfo.setTop(top)
}

func (ci *luaCallInfo) stackIndex(slot int) int {
	return ci.top() - len(ci.frame) + slot
}

func (ci *luaCallInfo) frameIndex(stackSlot int) int {
	if stackSlot < ci.top()-len(ci.frame) || ci.top() <= stackSlot {
		panic("frameIndex called with out-of-range stackSlot")
	}
	return stackSlot - ci.top() + len(ci.frame)
}

func (ci *luaCallInfo) base() int {
	return ci.stackIndex(0)
}

type goCallInfo struct {
	commonCallInfo
	context      int
	continuation Function
	/*
		oldErrorFunction ptrdiff_t
		extra ptrdiff_t
	*/
	oldAllowHook bool
	status       Status
}

func (ci *goCallInfo) isLua() bool {
	return false
}

func (l *State) pushLuaFrame(function, base, resultCount int, p *prototype) *luaCallInfo {
	ci, _ := l.callInfo.next().(*luaCallInfo)
	if ci == nil {
		ci = &luaCallInfo{}
		ci.previous_ = l.callInfo
		l.callInfo = l.callInfo.push(ci)
	} else {
		l.callInfo = ci
	}
	ci.initialize(l, function, base+p.maxStackSize, resultCount, callStatusLua)
	ci.frame = l.stack[base:ci.top()]
	ci.savedPC = 0
	ci.code = p.code
	l.top = ci.top()
	return ci
}

func (l *State) pushGoFrame(function, resultCount int) {
	ci, _ := l.callInfo.next().(*goCallInfo)
	if ci == nil {
		ci = &goCallInfo{}
		ci.previous_ = l.callInfo
		l.callInfo = l.callInfo.push(ci)
	} else {
		l.callInfo = ci
	}
	ci.initialize(l, function, l.top+MinStack, resultCount, 0)
}

func (ci *luaCallInfo) skip() {
	ci.savedPC++
}

func (ci *luaCallInfo) step() instruction {
	i := ci.code[ci.savedPC]
	ci.savedPC++
	return i
}

func (ci *luaCallInfo) jump(offset int) {
	ci.savedPC += pc(offset)
}

func (l *State) newLuaClosure(p *prototype) *luaClosure {
	return &luaClosure{prototype: p, upValues: make([]*upValue, len(p.upValues))}
}

func (l *State) findUpValue(level int) *upValue {
	for e := l.upValues; e != nil; e = e.next {
		if e.upValue.isInStackAt(level) {
			return e.upValue
		}
	}
	return l.newUpValueAt(level)
}

func (l *State) newClosure(p *prototype, upValues []*upValue, base int) value {
	c := l.newLuaClosure(p)
	p.cache = c
	for i, uv := range p.upValues {
		if uv.isLocal { // upValue refers to local variable
			c.upValues[i] = l.findUpValue(base + uv.index)
		} else { // get upValue from enclosing function
			c.upValues[i] = upValues[uv.index]
		}
	}
	return c
}

func cached(p *prototype, upValues []*upValue, base int) *luaClosure {
	c := p.cache
	if c != nil {
		for i, uv := range p.upValues {
			if uv.isLocal && !c.upValues[i].isInStackAt(base+uv.index) {
				return nil
			} else if !uv.isLocal && c.upValues[i].home != upValues[uv.index].home {
				return nil
			}
		}
	}
	return c
}

func (l *State) callGo(f value, function int, resultCount int) {
	l.checkStack(MinStack)
	l.pushGoFrame(function, resultCount)
	if l.hookMask&MaskCall != 0 {
		l.hook(HookCall, -1)
	}
	var n int
	switch f := f.(type) {
	case *goClosure:
		n = f.function(l)
	case Function:
		n = f(l)
	}
	ApiCheckStackSpace(l, n)
	l.postCall(l.top - n)
}

func (l *State) preCall(function int, resultCount int) bool {
	for {
		switch f := l.stack[function].(type) {
		case *goClosure:
			l.callGo(f, function, resultCount)
			return true
		case Function:
			l.callGo(f, function, resultCount)
			return true
		case *luaClosure:
			p := f.prototype
			l.checkStack(p.maxStackSize)
			argCount := l.top - function - 1
			args := l.stack[l.top : l.top+argCount]
			for i := range args {
				args[i] = nil
			}
			base := function + 1
			if p.isVarArg {
				base = l.adjustVarArgs(p, argCount)
			}
			ci := l.pushLuaFrame(function, base, resultCount, p)
			if l.hookMask&MaskCall != 0 {
				l.callHook(ci)
			}
			return false
		default:
			if tm := l.tagMethodByObject(f, tmCall); tm == nil {
				l.typeError(f, "call")
			} else if fun, ok := tm.(*luaClosure); !ok {
				l.typeError(f, "call")
			} else {
				// Slide the args + function up 1 slot and poke in the tag method
				for p := l.top; p > function; p-- {
					l.stack[p] = l.stack[p-1]
				}
				l.top++
				l.checkStack(0)
				l.stack[function] = fun
			}
		}
	}
	panic("unreachable")
}

func (l *State) callHook(ci *luaCallInfo) {
	ci.savedPC++ // hooks assume 'pc' is already incremented
	if pci, ok := ci.previous().(*luaCallInfo); ok && pci.code[pci.savedPC-1].opCode() == opTailCall {
		ci.setCallStatus(callStatusTail)
		l.hook(HookTailCall, -1)
	} else {
		l.hook(HookCall, -1)
	}
	ci.savedPC-- // correct 'pc'
}

func (l *State) adjustVarArgs(p *prototype, argCount int) int {
	fixedArgCount := p.parameterCount
	l.assert(argCount >= fixedArgCount)
	// move fixed parameters to final position
	fixed := l.top - argCount // first fixed argument
	base := l.top             // final position of first argument
	fixedArgs := l.stack[fixed : fixed+fixedArgCount]
	copy(l.stack[base:base+fixedArgCount], fixedArgs)
	for i := range fixedArgs {
		fixedArgs[i] = nil
	}
	return base
}

func (l *State) postCall(firstResult int) bool {
	ci := l.callInfo.(callInfo)
	if l.hookMask&MaskReturn != 0 {
		l.hook(HookReturn, -1)
	}
	result, wanted, available := ci.function(), ci.resultCount(), l.top-firstResult
	l.callInfo = ci.previous() // back to caller
	if available > wanted {
		available = wanted
	}
	if available > 0 { // copy available results to final position
		copy(l.stack[result:result+available], l.stack[firstResult:firstResult+available])
	}
	for i := result + available; i < result+wanted; i++ { // clear remaining results
		l.stack[i] = nil
	}
	l.top = result + wanted
	if l.hookMask&(MaskReturn|MaskLine) != 0 {
		l.oldPC = l.callInfo.(*luaCallInfo).savedPC // oldPC for caller function
	}
	return wanted != MultipleReturns
}

// Call a Go or Lua function. The function to be called is at function.
// The arguments are on the stack, right after the function. On return, all the
// results are on the stack, starting at the original function position.
func (l *State) call(function int, resultCount int, allowYield bool) {
	if l.nestedGoCallCount++; l.nestedGoCallCount == maxCallCount {
		l.runtimeError("Go stack overflow")
	} else if l.nestedGoCallCount >= maxCallCount+maxCallCount>>3 {
		l.throw(ErrorError) // error while handling stack error
	}
	if !allowYield {
		l.nonYieldableCallCount++
	}
	if !l.preCall(function, resultCount) { // is a Lua function?
		l.execute() // call it
	}
	if !allowYield {
		l.nonYieldableCallCount--
	}
	l.nestedGoCallCount--
}

func (l *State) throw(errorCode Status) {
	// TODO
	panic(errorCode)
}

func (l *State) hook(event, line int) {
	if l.hooker == nil || !l.allowHook {
		return
	}
	ci := l.callInfo.(*luaCallInfo)
	top := l.top
	ciTop := ci.top()
	ar := Debug{Event: event, CurrentLine: line, callInfo: ci}
	l.checkStack(MinStack)
	ci.setTop(l.top + MinStack)
	l.assert(ci.top() <= l.stackLast)
	l.allowHook = false // can't hook calls inside a hook
	ci.setCallStatus(callStatusHooked)
	l.hooker(l, &ar)
	l.assert(!l.allowHook)
	l.allowHook = true
	ci.setTop(ciTop)
	l.top = top
	ci.clearCallStatus(callStatusHooked)
}

func (l *State) initializeStack() {
	l.stack = make([]value, basicStackSize)
	l.stackLast = basicStackSize - extraStack
	l.top++
	ci := &l.baseCallInfo
	ci.frame = l.stack[:0]
	ci.setTop(l.top + MinStack)
	l.callInfo = ci
}

func (l *State) checkStack(n int) {
	if l.stackLast-l.top <= n {
		l.growStack(n)
	}
}

func (l *State) reallocStack(newSize int) {
	l.assert(newSize <= maxStack || newSize == errorStackSize)
	l.assert(l.stackLast == len(l.stack)-extraStack)
	l.stack = append(l.stack, make([]value, newSize-len(l.stack))...)
	l.stackLast = len(l.stack) - extraStack
	_ = l.callInfo.push(nil)
	for ci := l.callInfo; ci != nil; ci = ci.previous() {
		if lci, ok := ci.(*luaCallInfo); ok {
			top := lci.top()
			lci.frame = l.stack[top-len(lci.frame) : top]
		}
	}
}

func (l *State) growStack(n int) {
	if len(l.stack) > maxStack { // error after extra size?
		l.throw(ErrorError)
	} else {
		needed := l.top + n + extraStack
		newSize := 2 * len(l.stack)
		if newSize > maxStack {
			newSize = maxStack
		}
		if newSize < needed {
			newSize = needed
		}
		if newSize > maxStack { // stack overflow?
			l.reallocStack(errorStackSize)
			l.runtimeError("stack overflow")
		} else {
			l.reallocStack(newSize)
		}
	}
}
