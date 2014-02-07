package lua

import (
	"fmt"
	"math"
	"strings"
)

func (l *State) arith(rb, rc value, op tm) value {
	b, bok := toNumber(rb)
	c, cok := toNumber(rc)
	if bok && cok {
		return arith(Operator(op-tmAdd)+OpAdd, b, c)
	} else if result, ok := l.callBinaryTagMethod(rb, rc, op); ok {
		return result
	}
	l.arithError(rb, rc)
	return nil
}

func (l *State) tableAt(t value, key value) value {
	for loop := 0; loop < maxTagLoop; loop++ {
		var tm value
		if table, ok := t.(*table); ok {
			if result := table.at(key); result != nil {
				return result
			} else if tm = l.fastTagMethod(table.metaTable, tmIndex); tm == nil {
				return nil
			}
		} else if tm = l.tagMethodByObject(t, tmIndex); tm == nil {
			l.typeError(t, "index")
		}
		if f, ok := tm.(*luaClosure); ok {
			return l.callTagMethod(f, t, key)
		}
		t = tm
	}
	l.runtimeError("loop in table")
	return nil
}

func (l *State) setTableAt(t value, key value, val value) {
	for loop := 0; loop < maxTagLoop; loop++ {
		var tm value
		if table, ok := t.(*table); ok {
			if oldValue := table.at(key); oldValue != nil {
				// previous non-nil value ==> metamethod irrelevant
			} else if tm = l.fastTagMethod(table.metaTable, tmNewIndex); tm == nil {
				// no metamethod
			} else {
				ok = false
			}
			if ok {
				table.put(key, val)
				table.invalidateTagMethodCache()
				return
			}
		} else if tm = l.tagMethodByObject(t, tmNewIndex); tm == nil {
			l.typeError(t, "index")
		}
		switch tm.(type) {
		case *luaClosure:
		case *goClosure:
		case Function:
		default:
			t = tm
			continue
		}
		l.callTagMethodV(tm, t, key, val)
		return
	}
	l.runtimeError("loop in setTable")
}

func (l *State) objectLength(v value) value {
	var tm value
	switch v := v.(type) {
	case *table:
		if tm = l.fastTagMethod(v.metaTable, tmLen); tm == nil {
			return float64(v.length())
		}
	case string:
		return float64(len(v))
	default:
		if tm = l.tagMethodByObject(v, tmLen); tm == nil {
			l.typeError(v, "get length of")
		}
	}
	return l.callTagMethod(tm.(*luaClosure), v, v)
}

func (l *State) equalTagMethod(mt1, mt2 *table, event tm) (c *luaClosure) {
	if tm1 := l.fastTagMethod(mt1, event); tm1 == nil { // no metamethod
	} else if mt1 == mt2 { // same metatables => same metamethods
		c = tm1.(*luaClosure)
	} else if tm2 := l.fastTagMethod(mt2, event); tm2 == nil { // no metamethod
	} else if tm1 == tm2 { // same metamethods
		c = tm1.(*luaClosure)
	}
	return
}

func (l *State) equalObjects(t1, t2 value) bool {
	var tm *luaClosure
	switch t1 := t1.(type) {
	case *userData:
		if t1 == t2 {
			return true
		}
		tm = l.equalTagMethod(t1.metaTable, t2.(*userData).metaTable, tmEq)
	case *table:
		if t1 == t2 {
			return true
		}
		tm = l.equalTagMethod(t1.metaTable, t2.(*table).metaTable, tmEq)
	default:
		return t1 == t2
	}
	return tm != nil && !isFalse(l.callTagMethod(tm, t1, t2))
}

func (l *State) callBinaryTagMethod(p1, p2 value, event tm) (value, bool) {
	tm := l.tagMethodByObject(p1, event)
	if tm == nil {
		tm = l.tagMethodByObject(p2, event)
	}
	if tm == nil {
		return nil, false
	}
	return l.callTagMethod(tm.(*luaClosure), p1, p2), true
}

func (l *State) callOrderTagMethod(left, right value, event tm) (bool, bool) {
	result, ok := l.callBinaryTagMethod(left, right, event)
	return !isFalse(result), ok
}

func (l *State) lessThan(left, right value) bool {
	if lf, rf, ok := pairAsNumbers(left, right); ok {
		return lf < rf
	} else if ls, rs, ok := pairAsStrings(left, right); ok {
		return ls < rs
	} else if result, ok := l.callOrderTagMethod(left, right, tmLT); ok {
		return result
	}
	l.orderError(left, right)
	return false
}

func (l *State) lessOrEqual(left, right value) bool {
	if lf, rf, ok := pairAsNumbers(left, right); ok {
		return lf <= rf
	} else if ls, rs, ok := pairAsStrings(left, right); ok {
		return ls <= rs
	} else if result, ok := l.callOrderTagMethod(left, right, tmLE); ok {
		return result
	} else if result, ok := l.callOrderTagMethod(left, right, tmLT); ok {
		return result
	}
	l.orderError(left, right)
	return false
}

func (l *State) concat(total int) {
	t := func(i int) value { return l.stack[l.top-i] }
	put := func(i int, v value) { l.stack[l.top-i] = v }
	concatTagMethod := func() {
		if v, ok := l.callBinaryTagMethod(t(2), t(1), tmConcat); !ok {
			l.concatError(t(2), t(1))
		} else {
			put(2, v)
		}
	}
	l.assert(total >= 2)
	for total > 1 {
		top := l.top
		n := 2 // # of elements handled in this pass (at least 2)
		s2, ok := t(2).(string)
		if !ok {
			_, ok = t(2).(float64)
		}
		if !ok {
			concatTagMethod()
		} else if s1, ok := t(1).(string); !ok {
			concatTagMethod()
		} else if len(s1) == 0 {
			v, _ := toString(t(2))
			put(2, v)
		} else if s2, ok = t(2).(string); ok && len(s2) == 0 {
			put(2, t(1))
		} else {
			// at least 2 non-empty strings; scarf as many as possible
			ss := make([]string, 0, total)
			for i, ok, ss := 2, true, append(ss, s1); ok && i <= total; i++ {
				if s, ok := toString(l.stack[top-i]); ok {
					ss = append(ss, s)
				}
			}
			put(len(ss), strings.Join(ss, ""))
		}
		total -= n - 1 // created 1 new string from `n` strings
		l.top -= n - 1 // popped `n` strings and pushed 1
	}
}

func (l *State) traceExecution() {
	callInfo := l.callInfo.(*luaCallInfo)
	mask := l.hookMask
	countHook := mask&MaskCount != 0 && l.hookCount == 0
	if countHook {
		l.resetHookCount()
	}
	if callInfo.isCallStatus(callStatusHookYielded) {
		callInfo.clearCallStatus(callStatusHookYielded)
		return
	}
	if countHook {
		l.hook(HookCount, -1)
	}
	if mask&MaskLine != 0 {
		p := l.stack[callInfo.function()].(*luaClosure).prototype
		npc := callInfo.savedPC - 1
		newline := p.lineInfo[npc]
		if npc == 0 || callInfo.savedPC <= l.oldPC || newline != p.lineInfo[l.oldPC-1] {
			l.hook(HookLine, int(newline))
		}
	}
	l.oldPC = callInfo.savedPC
	if l.status == Yield {
		if countHook {
			l.hookCount = 1
		}
		callInfo.savedPC--
		callInfo.setCallStatus(callStatusHookYielded)
		callInfo.function_ = l.top - 1
		l.throw(Yield)
	}
}

func (l *State) execute() {
	var frame []value
	var closure *luaClosure
	var constants []value
	ci := l.callInfo.(*luaCallInfo)
	k := func(field int) value {
		if isConstant(field) {
			return constants[constantIndex(field)]
		}
		return frame[field]
	}
	jump := func(i instruction) {
		if a := i.a(); a > 0 {
			l.close(ci.stackIndex(a - 1))
		}
		ci.jump(i.sbx())
	}
	condJump := func(cond bool) {
		if cond {
			jump(ci.step())
		} else {
			ci.skip()
		}
	}
	expectNext := func(expected opCode) instruction {
		i := ci.step() // go to next instruction
		if op := i.opCode(); op != expected {
			panic(fmt.Sprintf("expected opcode %s, got %s", opNames[expected], opNames[op]))
		}
		return i
	}
	add := func(a, b float64) float64 { return a + b }
	sub := func(a, b float64) float64 { return a - b }
	mul := func(a, b float64) float64 { return a * b }
	div := func(a, b float64) float64 { return a / b }
	var i instruction
	arithOp := func(op func(float64, float64) float64, tm tm) {
		b := k(i.b())
		c := k(i.c())
		nb, bok := b.(float64)
		nc, cok := c.(float64)
		if bok && cok {
			frame[i.a()] = op(nb, nc)
		} else {
			frame[i.a()] = l.arith(b, c, tm)
			frame = ci.frame
		}
	}
	clear := func(r []value) {
		for i := range r {
			r[i] = nil
		}
	}
	newFrame := func() {
		l.assert(ci == l.callInfo)
		frame = ci.frame
		closure = l.stack[ci.function()].(*luaClosure)
		constants = closure.prototype.constants
	}
	newFrame()
	for {
		if l.hookMask&(MaskLine|MaskCount) != 0 {
			if l.hookCount--; l.hookCount == 0 || l.hookMask&MaskLine != 0 {
				l.traceExecution()
				frame = ci.frame
			}
		}
		switch i = ci.step(); i.opCode() {
		case opMove:
			frame[i.a()] = frame[i.b()]
		case opLoadConstant:
			frame[i.a()] = constants[i.bx()]
		case opLoadConstantEx:
			frame[i.a()] = constants[expectNext(opExtraArg).ax()]
		case opLoadBool:
			frame[i.a()] = i.b() != 0
			if i.c() != 0 {
				ci.skip()
			}
		case opLoadNil:
			a, b := i.a(), i.b()
			clear(frame[a : a+b+1])
		case opGetUpValue:
			frame[i.a()] = closure.upValue(i.b())
		case opGetTableUp:
			frame[i.a()] = l.tableAt(closure.upValue(i.b()), k(i.c()))
			frame = ci.frame
		case opGetTable:
			frame[i.a()] = l.tableAt(frame[i.b()], k(i.c()))
			frame = ci.frame
		case opSetTableUp:
			l.setTableAt(closure.upValue(i.a()), k(i.b()), k(i.c()))
			frame = ci.frame
		case opSetUpValue:
			closure.setUpValue(i.b(), frame[i.a()])
		case opSetTable:
			l.setTableAt(frame[i.a()], k(i.b()), k(i.c()))
			frame = ci.frame
		case opNewTable:
			a := i.a()
			if b, c := float8(i.b()), float8(i.c()); b != 0 || c != 0 {
				frame[a] = newTableWithSize(intFromFloat8(b), intFromFloat8(c))
			} else {
				frame[a] = newTable()
			}
			clear(frame[a+1:])
		case opSelf:
			a, t := i.a(), frame[i.b()]
			frame[a+1], frame[a] = t, l.tableAt(t, k(i.c()))
			frame = ci.frame
		case opAdd:
			arithOp(add, tmAdd)
		case opSub:
			arithOp(sub, tmSub)
		case opMul:
			arithOp(mul, tmMul)
		case opDiv:
			arithOp(div, tmDiv)
		case opMod:
			arithOp(math.Mod, tmMod)
		case opPow:
			arithOp(math.Pow, tmPow)
		case opUnaryMinus:
			switch b := frame[i.b()].(type) {
			case float64:
				frame[i.a()] = -b
			default:
				frame[i.a()] = l.arith(b, b, tmUnaryMinus)
				frame = ci.frame
			}
		case opNot:
			frame[i.a()] = isFalse(frame[i.b()])
		case opLength:
			frame[i.a()] = l.objectLength(frame[i.b()])
			frame = ci.frame
		case opConcat:
			a, b, c := i.a(), i.b(), i.c()
			l.top = ci.stackIndex(c + 1) // mark the end of concat operands
			l.concat(c - b + 1)
			frame = ci.frame
			frame[a] = frame[b]
			if a >= b { // limit of live values
				clear(frame[a+1:])
			} else {
				clear(frame[b:])
			}
		case opJump:
			jump(i)
		case opEqual:
			test := i.a() != 0
			condJump(l.equalObjects(k(i.b()), k(i.c())) == test)
			frame = ci.frame
		case opLessThan:
			test := i.a() != 0
			condJump(l.lessThan(k(i.b()), k(i.c())) == test)
			frame = ci.frame
		case opLessOrEqual:
			test := i.a() != 0
			condJump(l.lessOrEqual(k(i.b()), k(i.c())) == test)
			frame = ci.frame
		case opTest:
			test, t := i.c() != 0, !isFalse(frame[i.a()])
			condJump(test && t || !t)
		case opTestSet:
			b := frame[i.b()]
			if test, t := i.c() != 0, !isFalse(b); test && t || !t {
				frame[i.a()] = b
				jump(ci.step())
			} else {
				ci.skip()
			}
		case opCall:
			a, b, c := i.a(), i.b(), i.c()
			if b != 0 {
				l.top = ci.stackIndex(a + b)
			} // else previous instruction set top
			if n := c - 1; l.preCall(ci.stackIndex(a), n) { // go function
				if n >= 0 {
					l.top = ci.top() // adjust results
				}
				frame = ci.frame
			} else { // lua function
				ci = l.callInfo.(*luaCallInfo)
				ci.setCallStatus(callStatusReentry)
				newFrame()
			}
		case opTailCall:
			a, b, c := i.a(), i.b(), i.c()
			if b != 0 {
				l.top = ci.stackIndex(a + b)
			} // else previous instruction set top
			l.assert(c-1 == MultipleReturns)
			if l.preCall(ci.stackIndex(a), MultipleReturns) { // go function
				frame = ci.frame
			} else {
				// tail call: put called frame (n) in place of caller one (o)
				nci := l.callInfo.(*luaCallInfo)           // called frame
				oci := nci.previous().(*luaCallInfo)       // caller frame
				nfn, ofn := nci.function(), oci.function() // called & caller function
				// last stack slot filled by 'precall'
				lim := l.stack[nfn].(*luaClosure).prototype.parameterCount
				if len(closure.prototype.prototypes) > 0 { // close all upvalues from previous call
					l.close(oci.base())
				}
				// move new frame into old one
				copy(l.stack[ofn:ofn+lim+1], l.stack[nfn:nfn+lim+1])
				base := ofn + (nci.base() - nfn) // correct base
				oci.frame = l.stack[base:ci.top()]
				oci.top_ = ofn + (l.top - nfn) // correct top
				oci.savedPC = nci.savedPC
				oci.setCallStatus(callStatusTail) // function was tail called
				l.top, l.callInfo, ci = oci.top(), oci, oci
				l.assert(l.top == oci.base()+l.stack[ofn].(*luaClosure).prototype.maxStackSize)
				l.assert(&oci.frame[0] == &l.stack[oci.base()] && len(oci.frame) == oci.top()-oci.base())
				newFrame()
			}
		case opReturn:
			a := i.a()
			if b := i.b(); b != 0 {
				l.top = ci.stackIndex(a + b - 1)
			}
			if len(closure.prototype.prototypes) > 0 {
				l.close(ci.base())
			}
			n := l.postCall(ci.stackIndex(a))
			if !ci.isCallStatus(callStatusReentry) { // ci still the called one?
				return // external invocation: return
			}
			ci = l.callInfo.(*luaCallInfo)
			if n {
				l.top = ci.top()
			}
			l.assert(ci.code[ci.savedPC-1].opCode() == opCall)
			newFrame()
		case opForLoop:
			a := i.a()
			index, limit, step := frame[a+0].(float64), frame[a+1].(float64), frame[a+2].(float64)
			if index += step; 0 < step && index <= limit || limit <= index {
				ci.jump(i.sbx())
				frame[a+0] = index // update internal index...
				frame[a+3] = index // ... and external index
			}
		case opForPrep:
			a := i.a()
			if init, ok := toNumber(frame[a+0]); !ok {
				l.runtimeError("'for' initial value must be a number")
			} else if limit, ok := toNumber(frame[a+1]); !ok {
				l.runtimeError("'for' limit must be a number")
			} else if step, ok := toNumber(frame[a+2]); !ok {
				l.runtimeError("'for' step must be a number")
			} else {
				frame[a+0], frame[a+1], frame[a+2] = init-step, limit, step
				ci.jump(i.sbx())
			}
		case opTForCall:
			a := i.a()
			callBase := a + 3
			copy(frame[callBase:callBase+3], frame[a:a+3])
			l.top = callBase + 3 // function + 2 args (state and index)
			l.call(callBase, i.c(), true)
			frame, l.top = ci.frame, ci.top()
			i = expectNext(opTForLoop) // go to next instruction
			fallthrough
		case opTForLoop:
			if a := i.a(); frame[a+1] != nil { // continue loop?
				frame[a] = frame[a+1] // save control variable
				ci.jump(i.sbx())      // jump back
			}
		case opSetList:
			a, n, c := i.a(), i.b(), i.c()
			if n == 0 {
				n = l.top - a - 1
			}
			if c == 0 {
				c = expectNext(opExtraArg).ax()
			}
			h := frame[a].(*table)
			start := c - 1*listItemsPerFlush
			last := start + n
			if last > cap(h.array) {
				h.extendArray(last)
			}
			copy(h.array[start:last], frame[a:a+n])
			l.top = ci.top()
		case opClosure:
			a, p := i.a(), &closure.prototype.prototypes[i.bx()]
			if ncl := cached(p, closure.upValues, ci.base()); ncl == nil { // no match?
				frame[a] = l.newClosure(p, closure.upValues, ci.base()) // create a new one
			} else {
				frame[a] = ncl
			}
			clear(frame[a+1:])
		case opVarArg:
			a, b := i.a(), i.b()-1
			n := ci.base() - ci.function() - closure.prototype.parameterCount - 1
			if b < 0 {
				b = n // get all var arguments
				l.checkStack(n)
				frame = ci.frame
				l.top = ci.base() + a + n
			}
			for j := 0; j < b; j++ {
				if j < n {
					frame[a+j] = l.stack[ci.base()-n+j]
				} else {
					frame[a+j] = nil
				}
			}
		case opExtraArg:
			panic(fmt.Sprintf("unexpected opExtraArg instruction, '%s'", i.String()))
		}
	}
}
