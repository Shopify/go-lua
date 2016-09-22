package lua

import (
	"fmt"
	"math"
	"strings"
)

func (l *State) arith(rb, rc value, op tm) value {
	if b, ok := l.toNumber(rb); ok {
		if c, ok := l.toNumber(rc); ok {
			return arith(Operator(op-tmAdd)+OpAdd, b, c)
		}
	}
	if result, ok := l.callBinaryTagMethod(rb, rc, op); ok {
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
		switch tm.(type) {
		case closure, *goFunction:
			return l.callTagMethod(tm, t, key)
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
			if table.tryPut(l, key, val) {
				// previous non-nil value ==> metamethod irrelevant
				table.invalidateTagMethodCache()
				return
			} else if tm = l.fastTagMethod(table.metaTable, tmNewIndex); tm == nil {
				// no metamethod
				table.put(l, key, val)
				table.invalidateTagMethodCache()
				return
			}
		} else if tm = l.tagMethodByObject(t, tmNewIndex); tm == nil {
			l.typeError(t, "index")
		}
		switch tm.(type) {
		case closure, *goFunction:
			l.callTagMethodV(tm, t, key, val)
			return
		}
		t = tm
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
	return l.callTagMethod(tm, v, v)
}

func (l *State) equalTagMethod(mt1, mt2 *table, event tm) value {
	if tm1 := l.fastTagMethod(mt1, event); tm1 == nil { // no metamethod
	} else if mt1 == mt2 { // same metatables => same metamethods
		return tm1
	} else if tm2 := l.fastTagMethod(mt2, event); tm2 == nil { // no metamethod
	} else if tm1 == tm2 { // same metamethods
		return tm1
	}
	return nil
}

func (l *State) equalObjects(t1, t2 value) bool {
	var tm value
	switch t1 := t1.(type) {
	case *userData:
		if t1 == t2 {
			return true
		} else if t2, ok := t2.(*userData); ok {
			tm = l.equalTagMethod(t1.metaTable, t2.metaTable, tmEq)
		}
	case *table:
		if t1 == t2 {
			return true
		} else if t2, ok := t2.(*table); ok {
			tm = l.equalTagMethod(t1.metaTable, t2.metaTable, tmEq)
		}
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
	return l.callTagMethod(tm, p1, p2), true
}

func (l *State) callOrderTagMethod(left, right value, event tm) (bool, bool) {
	result, ok := l.callBinaryTagMethod(left, right, event)
	return !isFalse(result), ok
}

func (l *State) lessThan(left, right value) bool {
	if lf, ok := left.(float64); ok {
		if rf, ok := right.(float64); ok {
			return lf < rf
		}
	} else if ls, ok := left.(string); ok {
		if rs, ok := right.(string); ok {
			return ls < rs
		}
	}
	if result, ok := l.callOrderTagMethod(left, right, tmLT); ok {
		return result
	}
	l.orderError(left, right)
	return false
}

func (l *State) lessOrEqual(left, right value) bool {
	if lf, ok := left.(float64); ok {
		if rf, ok := right.(float64); ok {
			return lf <= rf
		}
	} else if ls, ok := left.(string); ok {
		if rs, ok := right.(string); ok {
			return ls <= rs
		}
	}
	if result, ok := l.callOrderTagMethod(left, right, tmLE); ok {
		return result
	} else if result, ok := l.callOrderTagMethod(right, left, tmLT); ok {
		return !result
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
		n := 2 // # of elements handled in this pass (at least 2)
		s2, ok := t(2).(string)
		if !ok {
			_, ok = t(2).(float64)
		}
		if !ok {
			concatTagMethod()
		} else if s1, ok := l.toString(l.top - 1); !ok {
			concatTagMethod()
		} else if len(s1) == 0 {
			v, _ := l.toString(l.top - 2)
			put(2, v)
		} else if s2, ok = t(2).(string); ok && len(s2) == 0 {
			put(2, t(1))
		} else {
			// at least 2 non-empty strings; scarf as many as possible
			ss := []string{s1}
			for ; n <= total; n++ {
				if s, ok := l.toString(l.top - n); ok {
					ss = append(ss, s)
				} else {
					break
				}
			}
			n-- // last increment wasn't valid
			for i, j := 0, len(ss)-1; i < j; i, j = i+1, j-1 {
				ss[i], ss[j] = ss[j], ss[i]
			}
			put(len(ss), strings.Join(ss, ""))
		}
		total -= n - 1 // created 1 new string from `n` strings
		l.top -= n - 1 // popped `n` strings and pushed 1
	}
}

func (l *State) traceExecution() {
	callInfo := l.callInfo
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
		p := l.prototype(callInfo)
		npc := callInfo.savedPC - 1
		newline := p.lineInfo[npc]
		if npc == 0 || callInfo.savedPC <= l.oldPC || newline != p.lineInfo[l.oldPC-1] {
			l.hook(HookLine, int(newline))
		}
	}
	l.oldPC = callInfo.savedPC
	if l.shouldYield {
		if countHook {
			l.hookCount = 1
		}
		callInfo.savedPC--
		callInfo.setCallStatus(callStatusHookYielded)
		callInfo.function = l.top - 1
		panic("Not implemented - use goroutines to emulate yield")
	}
}

type engine struct {
	frame     []value
	closure   *luaClosure
	constants []value
	callInfo  *callInfo
	l         *State
}

func (e *engine) k(field int) value {
	if 0 != field&bitRK { // OPT: Inline isConstant(field).
		return e.constants[field & ^bitRK] // OPT: Inline constantIndex(field).
	}
	return e.frame[field]
}

func (e *engine) expectNext(expected opCode) instruction {
	i := e.callInfo.step() // go to next instruction
	if op := i.opCode(); op != expected {
		panic(fmt.Sprintf("expected opcode %s, got %s", opNames[expected], opNames[op]))
	}
	return i
}

func clear(r []value) {
	for i := range r {
		r[i] = nil
	}
}

func (e *engine) newFrame() {
	ci := e.callInfo
	// if internalCheck {
	// 	e.l.assert(ci == e.l.callInfo.variant)
	// }
	e.frame = ci.frame
	e.closure = e.l.stack[ci.function].(*luaClosure)
	e.constants = e.closure.prototype.constants
}

func (e *engine) hooked() bool { return e.l.hookMask&(MaskLine|MaskCount) != 0 }

func (e *engine) hook() {
	if e.l.hookCount--; e.l.hookCount == 0 || e.l.hookMask&MaskLine != 0 {
		e.l.traceExecution()
		e.frame = e.callInfo.frame
	}
}

type engineOp func(*engine, instruction) (engineOp, instruction)

var jumpTable []engineOp

func init() {
	jumpTable = []engineOp{
		func(e *engine, i instruction) (engineOp, instruction) { // opMove
			e.frame[i.a()] = e.frame[i.b()]
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLoadConstant
			e.frame[i.a()] = e.constants[i.bx()]
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLoadConstantEx
			e.frame[i.a()] = e.constants[e.expectNext(opExtraArg).ax()]
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLoadBool
			e.frame[i.a()] = i.b() != 0
			if i.c() != 0 {
				e.callInfo.skip()
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLoadNil
			a, b := i.a(), i.b()
			clear(e.frame[a : a+b+1])
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opGetUpValue
			e.frame[i.a()] = e.closure.upValue(i.b())
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opGetTableUp
			tmp := e.l.tableAt(e.closure.upValue(i.b()), e.k(i.c()))
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opGetTable
			tmp := e.l.tableAt(e.frame[i.b()], e.k(i.c()))
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSetTableUp
			e.l.setTableAt(e.closure.upValue(i.a()), e.k(i.b()), e.k(i.c()))
			e.frame = e.callInfo.frame
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSetUpValue
			e.closure.setUpValue(i.b(), e.frame[i.a()])
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSetTable
			e.l.setTableAt(e.frame[i.a()], e.k(i.b()), e.k(i.c()))
			e.frame = e.callInfo.frame
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opNewTable
			a := i.a()
			if b, c := float8(i.b()), float8(i.c()); b != 0 || c != 0 {
				e.frame[a] = newTableWithSize(intFromFloat8(b), intFromFloat8(c))
			} else {
				e.frame[a] = newTable()
			}
			clear(e.frame[a+1:])
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSelf
			a, t := i.a(), e.frame[i.b()]
			tmp := e.l.tableAt(t, e.k(i.c()))
			e.frame = e.callInfo.frame
			e.frame[a+1], e.frame[a] = t, tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opAdd
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = nb + nc
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmAdd)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSub
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = nb - nc
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmSub)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opMul
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = nb * nc
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmMul)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opDiv
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = nb / nc
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmDiv)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opMod
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = math.Mod(nb, nc)
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmMod)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opPow
			b := e.k(i.b())
			c := e.k(i.c())
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					e.frame[i.a()] = math.Pow(nb, nc)
					if e.hooked() {
						e.hook()
					}
					i = e.callInfo.step()
					return jumpTable[i.opCode()], i
				}
			}
			tmp := e.l.arith(b, c, tmPow)
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opUnaryMinus
			switch b := e.frame[i.b()].(type) {
			case float64:
				e.frame[i.a()] = -b
			default:
				tmp := e.l.arith(b, b, tmUnaryMinus)
				e.frame = e.callInfo.frame
				e.frame[i.a()] = tmp
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opNot
			e.frame[i.a()] = isFalse(e.frame[i.b()])
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLength
			tmp := e.l.objectLength(e.frame[i.b()])
			e.frame = e.callInfo.frame
			e.frame[i.a()] = tmp
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opConcat
			a, b, c := i.a(), i.b(), i.c()
			e.l.top = e.callInfo.stackIndex(c + 1) // mark the end of concat operands
			e.l.concat(c - b + 1)
			e.frame = e.callInfo.frame
			e.frame[a] = e.frame[b]
			if a >= b { // limit of live values
				clear(e.frame[a+1:])
			} else {
				clear(e.frame[b:])
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opJump
			if a := i.a(); a > 0 {
				e.l.close(e.callInfo.stackIndex(a - 1))
			}
			e.callInfo.jump(i.sbx())
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opEqual
			test := i.a() != 0
			result := e.l.equalObjects(e.k(i.b()), e.k(i.c()))
			if result == test {
				i := e.callInfo.step()
				if a := i.a(); a > 0 {
					e.l.close(e.callInfo.stackIndex(a - 1))
				}
				e.callInfo.jump(i.sbx())
			} else {
				e.callInfo.skip()
			}
			e.frame = e.callInfo.frame
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLessThan
			test := i.a() != 0
			result := e.l.lessThan(e.k(i.b()), e.k(i.c()))
			if result == test {
				i := e.callInfo.step()
				if a := i.a(); a > 0 {
					e.l.close(e.callInfo.stackIndex(a - 1))
				}
				e.callInfo.jump(i.sbx())
			} else {
				e.callInfo.skip()
			}
			e.frame = e.callInfo.frame
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opLessOrEqual
			test := i.a() != 0
			result := e.l.lessOrEqual(e.k(i.b()), e.k(i.c()))
			if result == test {
				i := e.callInfo.step()
				if a := i.a(); a > 0 {
					e.l.close(e.callInfo.stackIndex(a - 1))
				}
				e.callInfo.jump(i.sbx())
			} else {
				e.callInfo.skip()
			}
			e.frame = e.callInfo.frame
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opTest
			test := i.c() == 0
			if isFalse(e.frame[i.a()]) == test {
				i := e.callInfo.step()
				if a := i.a(); a > 0 {
					e.l.close(e.callInfo.stackIndex(a - 1))
				}
				e.callInfo.jump(i.sbx())
			} else {
				e.callInfo.skip()
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opTestSet
			b := e.frame[i.b()]
			test := i.c() == 0
			if isFalse(b) == test {
				e.frame[i.a()] = b
				i := e.callInfo.step()
				if a := i.a(); a > 0 {
					e.l.close(e.callInfo.stackIndex(a - 1))
				}
				e.callInfo.jump(i.sbx())
			} else {
				e.callInfo.skip()
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opCall
			a, b, c := i.a(), i.b(), i.c()
			if b != 0 {
				e.l.top = e.callInfo.stackIndex(a + b)
			} // else previous instruction set top
			if n := c - 1; e.l.preCall(e.callInfo.stackIndex(a), n) { // go function
				if n >= 0 {
					e.l.top = e.callInfo.top // adjust results
				}
				e.frame = e.callInfo.frame
			} else { // lua function
				e.callInfo = e.l.callInfo
				e.callInfo.setCallStatus(callStatusReentry)
				e.newFrame()
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opTailCall
			a, b := i.a(), i.b()
			if b != 0 {
				e.l.top = e.callInfo.stackIndex(a + b)
			} // else previous instruction set top
			// TODO e.l.assert(i.c()-1 == MultipleReturns)
			if e.l.preCall(e.callInfo.stackIndex(a), MultipleReturns) { // go function
				e.frame = e.callInfo.frame
			} else {
				// tail call: put called frame (n) in place of caller one (o)
				nci := e.l.callInfo                    // called frame
				oci := nci.previous                    // caller frame
				nfn, ofn := nci.function, oci.function // called & caller function
				// last stack slot filled by 'precall'
				lim := nci.base() + e.l.stack[nfn].(*luaClosure).prototype.parameterCount
				if len(e.closure.prototype.prototypes) > 0 { // close all upvalues from previous call
					e.l.close(oci.base())
				}
				// move new frame into old one
				for i := 0; nfn+i < lim; i++ {
					e.l.stack[ofn+i] = e.l.stack[nfn+i]
				}
				base := ofn + (nci.base() - nfn)  // correct base
				oci.setTop(ofn + (e.l.top - nfn)) // correct top
				oci.frame = e.l.stack[base:oci.top]
				oci.savedPC, oci.code = nci.savedPC, nci.code // correct code (savedPC indexes nci->code)
				oci.setCallStatus(callStatusTail)             // function was tail called
				e.l.top, e.l.callInfo, e.callInfo = oci.top, oci, oci
				// TODO e.l.assert(e.l.top == oci.base()+e.l.stack[ofn].(*luaClosure).prototype.maxStackSize)
				// TODO e.l.assert(&oci.frame[0] == &e.l.stack[oci.base()] && len(oci.frame) == oci.top-oci.base())
				e.newFrame()
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opReturn
			a := i.a()
			if b := i.b(); b != 0 {
				e.l.top = e.callInfo.stackIndex(a + b - 1)
			}
			if len(e.closure.prototype.prototypes) > 0 {
				e.l.close(e.callInfo.base())
			}
			n := e.l.postCall(e.callInfo.stackIndex(a))
			if !e.callInfo.isCallStatus(callStatusReentry) { // ci still the called one?
				return nil, i // external invocation: return
			}
			e.callInfo = e.l.callInfo
			if n {
				e.l.top = e.callInfo.top
			}
			// TODO l.assert(e.callInfo.code[e.callInfo.savedPC-1].opCode() == opCall)
			e.newFrame()
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opForLoop
			a := i.a()
			index, limit, step := e.frame[a+0].(float64), e.frame[a+1].(float64), e.frame[a+2].(float64)
			if index += step; (0 < step && index <= limit) || (step <= 0 && limit <= index) {
				e.callInfo.jump(i.sbx())
				e.frame[a+0] = index // update internal index...
				e.frame[a+3] = index // ... and external index
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opForPrep
			a := i.a()
			if init, ok := e.l.toNumber(e.frame[a+0]); !ok {
				e.l.runtimeError("'for' initial value must be a number")
			} else if limit, ok := e.l.toNumber(e.frame[a+1]); !ok {
				e.l.runtimeError("'for' limit must be a number")
			} else if step, ok := e.l.toNumber(e.frame[a+2]); !ok {
				e.l.runtimeError("'for' step must be a number")
			} else {
				e.frame[a+0], e.frame[a+1], e.frame[a+2] = init-step, limit, step
				e.callInfo.jump(i.sbx())
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opTForCall
			a := i.a()
			callBase := a + 3
			copy(e.frame[callBase:callBase+3], e.frame[a:a+3])
			callBase += e.callInfo.base()
			e.l.top = callBase + 3 // function + 2 args (state and index)
			e.l.call(callBase, i.c(), true)
			e.frame, e.l.top = e.callInfo.frame, e.callInfo.top
			i = e.expectNext(opTForLoop)         // go to next instruction
			if a := i.a(); e.frame[a+1] != nil { // continue loop?
				e.frame[a] = e.frame[a+1] // save control variable
				e.callInfo.jump(i.sbx())  // jump back
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opTForLoop:
			if a := i.a(); e.frame[a+1] != nil { // continue loop?
				e.frame[a] = e.frame[a+1] // save control variable
				e.callInfo.jump(i.sbx())  // jump back
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opSetList:
			a, n, c := i.a(), i.b(), i.c()
			if n == 0 {
				n = e.l.top - e.callInfo.stackIndex(a) - 1
			}
			if c == 0 {
				c = e.expectNext(opExtraArg).ax()
			}
			h := e.frame[a].(*table)
			start := (c - 1) * listItemsPerFlush
			last := start + n
			if last > len(h.array) {
				h.extendArray(last)
			}
			copy(h.array[start:last], e.frame[a+1:a+1+n])
			e.l.top = e.callInfo.top
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opClosure
			a, p := i.a(), &e.closure.prototype.prototypes[i.bx()]
			if ncl := cached(p, e.closure.upValues, e.callInfo.base()); ncl == nil { // no match?
				e.frame[a] = e.l.newClosure(p, e.closure.upValues, e.callInfo.base()) // create a new one
			} else {
				e.frame[a] = ncl
			}
			clear(e.frame[a+1:])
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opVarArg
			ci := e.callInfo
			a, b := i.a(), i.b()-1
			n := ci.base() - ci.function - e.closure.prototype.parameterCount - 1
			if b < 0 {
				b = n // get all var arguments
				e.l.checkStack(n)
				e.l.top = ci.base() + a + n
				if ci.top < e.l.top {
					ci.setTop(e.l.top)
					ci.frame = e.l.stack[ci.base():ci.top]
				}
				e.frame = ci.frame
			}
			for j := 0; j < b; j++ {
				if j < n {
					e.frame[a+j] = e.l.stack[ci.base()-n+j]
				} else {
					e.frame[a+j] = nil
				}
			}
			if e.hooked() {
				e.hook()
			}
			i = e.callInfo.step()
			return jumpTable[i.opCode()], i
		},
		func(e *engine, i instruction) (engineOp, instruction) { // opExtraArg
			panic(fmt.Sprintf("unexpected opExtraArg instruction, '%s'", i.String()))
		},
	}
}

func (l *State) execute() { l.executeFunctionTable() }

func (l *State) executeFunctionTable() {
	ci := l.callInfo
	closure, _ := l.stack[ci.function].(*luaClosure)
	e := engine{callInfo: ci, frame: ci.frame, closure: closure, constants: closure.prototype.constants, l: l}
	if l.hookMask&(MaskLine|MaskCount) != 0 {
		if l.hookCount--; l.hookCount == 0 || l.hookMask&MaskLine != 0 {
			l.traceExecution()
			e.frame = e.callInfo.frame
		}
	}
	i := e.callInfo.step()
	f := jumpTable[i.opCode()]
	for f, i = f(&e, i); f != nil; f, i = f(&e, i) {
	}
}

func k(field int, constants []value, frame []value) value {
	if 0 != field&bitRK { // OPT: Inline isConstant(field).
		return constants[field & ^bitRK] // OPT: Inline constantIndex(field).
	}
	return frame[field]
}

func newFrame(l *State, ci *callInfo) (frame []value, closure *luaClosure, constants []value) {
	// TODO l.assert(ci == l.callInfo)
	frame = ci.frame
	closure, _ = l.stack[ci.function].(*luaClosure)
	constants = closure.prototype.constants
	return
}

func expectNext(ci *callInfo, expected opCode) instruction {
	i := ci.step() // go to next instruction
	if op := i.opCode(); op != expected {
		panic(fmt.Sprintf("expected opcode %s, got %s", opNames[expected], opNames[op]))
	}
	return i
}

func (l *State) executeSwitch() {
	ci := l.callInfo
	frame, closure, constants := newFrame(l, ci)
	for {
		if l.hookMask&(MaskLine|MaskCount) != 0 {
			if l.hookCount--; l.hookCount == 0 || l.hookMask&MaskLine != 0 {
				l.traceExecution()
				frame = ci.frame
			}
		}
		switch i := ci.step(); i.opCode() {
		case opMove:
			frame[i.a()] = frame[i.b()]
		case opLoadConstant:
			frame[i.a()] = constants[i.bx()]
		case opLoadConstantEx:
			frame[i.a()] = constants[expectNext(ci, opExtraArg).ax()]
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
			tmp := l.tableAt(closure.upValue(i.b()), k(i.c(), constants, frame))
			frame = ci.frame
			frame[i.a()] = tmp
		case opGetTable:
			tmp := l.tableAt(frame[i.b()], k(i.c(), constants, frame))
			frame = ci.frame
			frame[i.a()] = tmp
		case opSetTableUp:
			l.setTableAt(closure.upValue(i.a()), k(i.b(), constants, frame), k(i.c(), constants, frame))
			frame = ci.frame
		case opSetUpValue:
			closure.setUpValue(i.b(), frame[i.a()])
		case opSetTable:
			l.setTableAt(frame[i.a()], k(i.b(), constants, frame), k(i.c(), constants, frame))
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
			tmp := l.tableAt(t, k(i.c(), constants, frame))
			frame = ci.frame
			frame[a+1], frame[a] = t, tmp
		case opAdd:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = nb + nc
					break
				}
			}
			tmp := l.arith(b, c, tmAdd)
			frame = ci.frame
			frame[i.a()] = tmp
		case opSub:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = nb - nc
					break
				}
			}
			tmp := l.arith(b, c, tmSub)
			frame = ci.frame
			frame[i.a()] = tmp
		case opMul:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = nb * nc
					break
				}
			}
			tmp := l.arith(b, c, tmMul)
			frame = ci.frame
			frame[i.a()] = tmp
		case opDiv:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = nb / nc
					break
				}
			}
			tmp := l.arith(b, c, tmDiv)
			frame = ci.frame
			frame[i.a()] = tmp
		case opMod:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = math.Mod(nb, nc)
					break
				}
			}
			tmp := l.arith(b, c, tmMod)
			frame = ci.frame
			frame[i.a()] = tmp
		case opPow:
			b := k(i.b(), constants, frame)
			c := k(i.c(), constants, frame)
			if nb, ok := b.(float64); ok {
				if nc, ok := c.(float64); ok {
					frame[i.a()] = math.Pow(nb, nc)
					break
				}
			}
			tmp := l.arith(b, c, tmPow)
			frame = ci.frame
			frame[i.a()] = tmp
		case opUnaryMinus:
			switch b := frame[i.b()].(type) {
			case float64:
				frame[i.a()] = -b
			default:
				tmp := l.arith(b, b, tmUnaryMinus)
				frame = ci.frame
				frame[i.a()] = tmp
			}
		case opNot:
			frame[i.a()] = isFalse(frame[i.b()])
		case opLength:
			tmp := l.objectLength(frame[i.b()])
			frame = ci.frame
			frame[i.a()] = tmp
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
			if a := i.a(); a > 0 {
				l.close(ci.stackIndex(a - 1))
			}
			ci.jump(i.sbx())
		case opEqual:
			test := i.a() != 0
			if l.equalObjects(k(i.b(), constants, frame), k(i.c(), constants, frame)) == test {
				i := ci.step()
				if a := i.a(); a > 0 {
					l.close(ci.stackIndex(a - 1))
				}
				ci.jump(i.sbx())
			} else {
				ci.skip()
			}
			frame = ci.frame
		case opLessThan:
			test := i.a() != 0
			if l.lessThan(k(i.b(), constants, frame), k(i.c(), constants, frame)) == test {
				i := ci.step()
				if a := i.a(); a > 0 {
					l.close(ci.stackIndex(a - 1))
				}
				ci.jump(i.sbx())
			} else {
				ci.skip()
			}
			frame = ci.frame
		case opLessOrEqual:
			test := i.a() != 0
			if l.lessOrEqual(k(i.b(), constants, frame), k(i.c(), constants, frame)) == test {
				i := ci.step()
				if a := i.a(); a > 0 {
					l.close(ci.stackIndex(a - 1))
				}
				ci.jump(i.sbx())
			} else {
				ci.skip()
			}
			frame = ci.frame
		case opTest:
			test := i.c() == 0
			if isFalse(frame[i.a()]) == test {
				i := ci.step()
				if a := i.a(); a > 0 {
					l.close(ci.stackIndex(a - 1))
				}
				ci.jump(i.sbx())
			} else {
				ci.skip()
			}
		case opTestSet:
			b := frame[i.b()]
			test := i.c() == 0
			if isFalse(b) == test {
				frame[i.a()] = b
				i := ci.step()
				if a := i.a(); a > 0 {
					l.close(ci.stackIndex(a - 1))
				}
				ci.jump(i.sbx())
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
					l.top = ci.top // adjust results
				}
				frame = ci.frame
			} else { // lua function
				ci = l.callInfo
				ci.setCallStatus(callStatusReentry)
				frame, closure, constants = newFrame(l, ci)
			}
		case opTailCall:
			a, b := i.a(), i.b()
			if b != 0 {
				l.top = ci.stackIndex(a + b)
			} // else previous instruction set top
			// TODO l.assert(i.c()-1 == MultipleReturns)
			if l.preCall(ci.stackIndex(a), MultipleReturns) { // go function
				frame = ci.frame
			} else {
				// tail call: put called frame (n) in place of caller one (o)
				nci := l.callInfo                      // called frame
				oci := nci.previous                    // caller frame
				nfn, ofn := nci.function, oci.function // called & caller function
				// last stack slot filled by 'precall'
				lim := nci.base() + l.stack[nfn].(*luaClosure).prototype.parameterCount
				if len(closure.prototype.prototypes) > 0 { // close all upvalues from previous call
					l.close(oci.base())
				}
				// move new frame into old one
				for i := 0; nfn+i < lim; i++ {
					l.stack[ofn+i] = l.stack[nfn+i]
				}
				base := ofn + (nci.base() - nfn) // correct base
				oci.setTop(ofn + (l.top - nfn))  // correct top
				oci.frame = l.stack[base:oci.top]
				oci.savedPC, oci.code = nci.savedPC, nci.code // correct code (savedPC indexes nci->code)
				oci.setCallStatus(callStatusTail)             // function was tail called
				l.top, l.callInfo, ci = oci.top, oci, oci
				// TODO l.assert(l.top == oci.base()+l.stack[ofn].(*luaClosure).prototype.maxStackSize)
				// TODO l.assert(&oci.frame[0] == &l.stack[oci.base()] && len(oci.frame) == oci.top-oci.base())
				frame, closure, constants = newFrame(l, ci)
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
			ci = l.callInfo
			if n {
				l.top = ci.top
			}
			// TODO l.assert(ci.code[ci.savedPC-1].opCode() == opCall)
			frame, closure, constants = newFrame(l, ci)
		case opForLoop:
			a := i.a()
			index, limit, step := frame[a+0].(float64), frame[a+1].(float64), frame[a+2].(float64)
			if index += step; (0 < step && index <= limit) || (step <= 0 && limit <= index) {
				ci.jump(i.sbx())
				frame[a+0] = index // update internal index...
				frame[a+3] = index // ... and external index
			}
		case opForPrep:
			a := i.a()
			if init, ok := l.toNumber(frame[a+0]); !ok {
				l.runtimeError("'for' initial value must be a number")
			} else if limit, ok := l.toNumber(frame[a+1]); !ok {
				l.runtimeError("'for' limit must be a number")
			} else if step, ok := l.toNumber(frame[a+2]); !ok {
				l.runtimeError("'for' step must be a number")
			} else {
				frame[a+0], frame[a+1], frame[a+2] = init-step, limit, step
				ci.jump(i.sbx())
			}
		case opTForCall:
			a := i.a()
			callBase := a + 3
			copy(frame[callBase:callBase+3], frame[a:a+3])
			callBase += ci.base()
			l.top = callBase + 3 // function + 2 args (state and index)
			l.call(callBase, i.c(), true)
			frame, l.top = ci.frame, ci.top
			i = expectNext(ci, opTForLoop) // go to next instruction
			fallthrough
		case opTForLoop:
			if a := i.a(); frame[a+1] != nil { // continue loop?
				frame[a] = frame[a+1] // save control variable
				ci.jump(i.sbx())      // jump back
			}
		case opSetList:
			a, n, c := i.a(), i.b(), i.c()
			if n == 0 {
				n = l.top - ci.stackIndex(a) - 1
			}
			if c == 0 {
				c = expectNext(ci, opExtraArg).ax()
			}
			h := frame[a].(*table)
			start := (c - 1) * listItemsPerFlush
			last := start + n
			if last > len(h.array) {
				h.extendArray(last)
			}
			copy(h.array[start:last], frame[a+1:a+1+n])
			l.top = ci.top
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
			n := ci.base() - ci.function - closure.prototype.parameterCount - 1
			if b < 0 {
				b = n // get all var arguments
				l.checkStack(n)
				l.top = ci.base() + a + n
				if ci.top < l.top {
					ci.setTop(l.top)
					ci.frame = l.stack[ci.base():ci.top]
				}
				frame = ci.frame
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
