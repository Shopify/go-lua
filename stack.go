package lua

func (l *state) push(v value) {
	l.stack[l.top] = v
	l.top++
}

func (l *state) pop() value {
	l.top--
	return l.stack[l.top]
}

type upValue struct {
	home interface{}
}

type closure interface {
	upValue(i int) value
	setUpValue(i int, v value)
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

func (c *goClosure) upValue(i int) value {
	return c.upValues[i]
}

func (c *goClosure) setUpValue(i int, v value) {
	c.upValues[i] = v
}

func (l *state) newUpValue() *upValue {
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

func (uv *upValue) isInStackAt(level stackIndex) bool {
	if home, ok := uv.home.(stackLocation); ok {
		return home.index == level
	}
	return false
}

func (uv *upValue) isInStackBelow(level stackIndex) bool {
	if home, ok := uv.home.(stackLocation); ok {
		return home.index < level
	}
	return false
}

type openUpValue struct {
	upValue *upValue
	next    *openUpValue
}

func (l *state) newUpValueAt(level stackIndex) *upValue {
	uv := &upValue{home: stackLocation{state: l, index: level}}
	l.upValues = &openUpValue{upValue: uv, next: l.upValues}
	return uv
}

func (l *state) close(level stackIndex) {
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
	function() stackIndex
	top() stackIndex
	setTop(stackIndex)
}

type commonCallInfo struct {
	function_        stackIndex
	top_             stackIndex
	previous_, next_ callInfo
	resultCount      int16
	callStatus       callStatus
}

func (ci *commonCallInfo) top() stackIndex {
	return ci.top_
}

func (ci *commonCallInfo) setTop(top stackIndex) {
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

func (ci *commonCallInfo) function() stackIndex {
	return ci.function_
}

func (ci *commonCallInfo) initialize(l *state, function, top stackIndex, resultCount int16, callStatus callStatus) {
	ci.function_ = function
	ci.top_ = top
	l.assert(ci.top() <= l.stackLast)
	ci.resultCount = resultCount
	ci.callStatus = callStatus
}

type luaCallInfo struct {
	commonCallInfo
	frame   []value
	base    stackIndex
	savedPC pc
	code    []instruction
}

type goCallInfo struct {
	commonCallInfo
	context int
	/*
		continuation goFunction
		oldErrorFunction ptrdiff_t
		extra ptrdiff_t
	*/
	oldAllowHook bool
	status       byte
}

func (l *state) pushLuaFrame(function, base stackIndex, resultCount int16, p *prototype) *luaCallInfo {
	ci, _ := l.callInfo.next().(*luaCallInfo)
	if ci == nil {
		ci = &luaCallInfo{}
		ci.previous_ = l.callInfo
		l.callInfo = l.callInfo.push(ci)
	} else {
		l.callInfo = ci
	}
	ci.initialize(l, function, base+stackIndex(p.maxStackSize), resultCount, callStatusLua)
	ci.frame = l.stack[base:ci.top()]
	ci.base = base
	ci.savedPC = 0
	ci.code = p.code
	l.top = ci.top()
	return ci
}

func (l *state) pushGoFrame(function stackIndex, resultCount int16) {
	ci, _ := l.callInfo.next().(*goCallInfo)
	if ci == nil {
		ci = &goCallInfo{}
		ci.previous_ = l.callInfo
		l.callInfo = l.callInfo.push(ci)
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

func (ci *luaCallInfo) jump(offset opField) {
	ci.savedPC += pc(offset)
}

func (l *state) newLuaClosure(p *prototype) *luaClosure {
	return &luaClosure{prototype: p, upValues: make([]*upValue, len(p.upValues))}
}

func (l *state) findUpValue(level stackIndex) *upValue {
	for e := l.upValues; e != nil; e = e.next {
		if e.upValue.isInStackAt(level) {
			return e.upValue
		}
	}
	return l.newUpValueAt(level)
}

func (l *state) newClosure(p *prototype, upValues []*upValue, base stackIndex) value {
	c := l.newLuaClosure(p)
	p.cache = c
	for i, uv := range p.upValues {
		if uv.isLocal { // upValue refers to local variable
			c.upValues[i] = l.findUpValue(base + stackIndex(uv.index))
		} else { // get upValue from enclosing function
			c.upValues[i] = upValues[uv.index]
		}
	}
	return c
}

func cached(p *prototype, upValues []*upValue, base stackIndex) *luaClosure {
	c := p.cache
	if c != nil {
		for i, uv := range p.upValues {
			if uv.isLocal && !c.upValues[i].isInStackAt(base+stackIndex(uv.index)) {
				return nil
			} else if !uv.isLocal && c.upValues[i].home != upValues[uv.index].home {
				return nil
			}
		}
	}
	return c
}

func (l *state) preCall(function stackIndex, resultCount int16) bool {
	for {
		switch f := l.stack[function].(type) {
		case *goClosure:
			l.checkStack(MinStack)
			l.pushGoFrame(function, resultCount)
			if l.hookMask&MaskCall != 0 {
				l.hook(HookCall, -1)
			}
			n := f.function(l)
			l.ApiCheckStackSpace(n)
			l.postCall(l.top - stackIndex(n))
			return true
		case *luaClosure:
			p := f.prototype
			l.checkStack(int(p.maxStackSize))
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

func (l *state) callHook(ci *luaCallInfo) {
	ci.savedPC++ // hooks assume 'pc' is already incremented
	if pci, ok := ci.previous().(*luaCallInfo); ok && pci.code[pci.savedPC-1].opCode() == opTailCall {
		ci.callStatus |= callStatusTail
		l.hook(HookTailCall, -1)
	} else {
		l.hook(HookCall, -1)
	}
	ci.savedPC-- // correct 'pc'
}

func (l *state) adjustVarArgs(p *prototype, argCount stackIndex) stackIndex {
	fixedArgCount := stackIndex(p.parameterCount)
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

func (l *state) postCall(firstResult stackIndex) bool {
	ci := l.callInfo.(*luaCallInfo)
	if l.hookMask&MaskReturn != 0 {
		l.hook(HookReturn, -1)
	}
	result := ci.function() // final position of first result
	wanted := stackIndex(ci.resultCount)
	if base, limit := firstResult, firstResult+wanted; l.top > limit {
		copy(l.stack[result:result+wanted], l.stack[base:limit])
	} else {
		copy(l.stack[result:result+wanted], l.stack[base:l.top])
		results := l.stack[result+wanted-(limit-l.top) : result+wanted]
		for i := range results {
			results[i] = nil
		}
	}
	l.top = result
	l.callInfo = ci.previous() // back to caller
	if l.hookMask&MaskReturn|MaskLine != 0 {
		l.oldPC = l.callInfo.(*luaCallInfo).savedPC // oldPC for caller function
	}
	return wanted != MultipleReturns
}

// Call a Go or Lua function. The function to be called is at function.
// The arguments are on the stack, right after the function. On return, all the
// results are on the stack, starting at the original function position.
func (l *state) call(function stackIndex, resultCount int16, allowYield bool) {
	if l.nestedGoCallCount++; l.nestedGoCallCount == maxCallCount {
		l.runtimeError("Go stack overflow")
	} else if l.nestedGoCallCount >= maxCallCount+maxCallCount>>3 {
		l.throw(Error) // error while handling stack error
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

func (l *state) throw(errorCode int) {
	// TODO
	panic(errorCode)
}

func (l *state) hook(event, line int) {
	if l.hooker == nil || !l.allowHook {
		return
	}
	ci := l.callInfo.(*luaCallInfo)
	top := l.top
	ciTop := ci.top()
	ar := Debug{Event: event, CurrentLine: line, callInfo: ci}
	l.checkStack(MinStack)
	ci.top_ = l.top + MinStack
	l.assert(ci.top() <= l.stackLast)
	l.allowHook = false // can't hook calls inside a hook
	ci.callStatus |= callStatusHooked
	l.hooker(l, &ar)
	l.assert(!l.allowHook)
	l.allowHook = true
	ci.top_ = ciTop
	l.top = top
	ci.callStatus &^= callStatusHooked
}

func (l *state) initializeStack() {
	l.stack = make([]value, basicStackSize)
	l.stackLast = basicStackSize - extraStack
	l.top++
	ci := &l.baseCallInfo
	ci.top_ = l.top + MinStack
	l.callInfo = ci
}

func (l *state) checkStack(n int) {
	if l.stackLast-l.top <= stackIndex(n) {
		l.growStack(n)
	}
}

func (l *state) reallocStack(newSize int) {
	l.assert(newSize <= maxStack || newSize == errorStackSize)
	l.assert(int(l.stackLast) == len(l.stack)-extraStack)
	l.stack = append(l.stack, make([]value, newSize-len(l.stack))...)
	l.stackLast = stackIndex(len(l.stack) - extraStack)
	_ = l.callInfo.push(nil)
	for ci := l.callInfo; ci != nil; ci = ci.previous() {
		if lci, ok := ci.(*luaCallInfo); ok {
			lci.frame = l.stack[lci.base : lci.base+stackIndex(len(lci.frame))]
		}
	}
}

func (l *state) growStack(n int) {
	if len(l.stack) > maxStack { // error after extra size?
		l.throw(Error)
	} else {
		needed := int(l.top) + n + extraStack
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
