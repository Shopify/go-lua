package lua

import (
	"fmt"
	"math"
)

const (
	oprMinus = iota
	oprNot
	oprLength
	oprNoUnary
)

const (
	noJump            = -1
	noRegister        = maxArgA
	maxLocalVariables = 200
)

const (
	oprAdd = iota
	oprSub
	oprMul
	oprDiv
	oprMod
	oprPow
	oprConcat
	oprEq
	oprLT
	oprLE
	oprNE
	oprGT
	oprGE
	oprAnd
	oprOr
	oprNoBinary
)

const (
	kindVoid = iota // no value
	kindNil
	kindTrue
	kindFalse
	kindConstant       // info = index of constant
	kindNumber         // value = numerical value
	kindNonRelocatable // info = result register
	kindLocal          // info = local register
	kindUpValue        // info = index of upvalue
	kindIndexed        // table = table register/upvalue, index = register/constant index
	kindJump           // info = instruction pc
	kindRelocatable    // info = instruction pc
	kindCall           // info = instruction pc
	kindVarArg         // info = instruction pc
)

// var kinds []string = []string{
// 	"void",
// 	"nil",
// 	"true",
// 	"false",
// 	"constant",
// 	"number",
// 	"nonrelocatable",
// 	"local",
// 	"upvalue",
// 	"indexed",
// 	"jump",
// 	"relocatable",
// 	"call",
// 	"vararg",
// }

type exprDesc struct {
	kind      int
	index     int // register/constant index
	table     int // register or upvalue
	tableType int // whether 'table' is register (kindLocal) or upvalue (kindUpValue)
	info      int
	t, f      int // patch lists for 'exit when true/false'
	value     float64
}

type assignmentTarget struct {
	previous *assignmentTarget
	exprDesc
}

type label struct {
	name                string
	pc, line            int
	activeVariableCount int
}

type block struct {
	previous              *block
	firstLabel, firstGoto int
	activeVariableCount   int
	hasUpValue, isLoop    bool
}

type function struct {
	f                      *prototype
	h                      *table
	previous               *function
	p                      *parser
	block                  *block
	pc, jumpPC, lastTarget int
	freeRegisterCount      int
	activeVariableCount    int
	firstLocal             int
}

func (f *function) OpenFunction(line int) {
	f.f.prototypes = append(f.f.prototypes, prototype{source: f.p.source, maxStackSize: 2, lineDefined: line})
	f.p.function = &function{f: &f.f.prototypes[len(f.f.prototypes)-1], h: newTable(), previous: f, p: f.p, jumpPC: noJump, firstLocal: len(f.p.activeVariables)}
	f.p.function.EnterBlock(false)
}

func (f *function) CloseFunction() exprDesc {
	e := f.previous.ExpressionToNextRegister(makeExpression(kindRelocatable, f.previous.EncodeABx(opClosure, 0, len(f.previous.f.prototypes)-1)))
	f.ReturnNone()
	f.LeaveBlock()
	f.assert(f.block == nil)
	f.p.function = f.previous
	return e
}

func (f *function) EnterBlock(isLoop bool) {
	// TODO www.lua.org uses a trick here to stack allocate the block, and chain blocks in the stack
	f.block = &block{previous: f.block, firstLabel: len(f.p.activeLabels), firstGoto: len(f.p.pendingGotos), activeVariableCount: f.activeVariableCount, isLoop: isLoop}
	f.assert(f.freeRegisterCount == f.activeVariableCount)
}

func (f *function) undefinedGotoError(g label) {
	if isReserved(g.name) {
		f.semanticError(fmt.Sprintf("<%s> at line %d not inside a loop", g.name, g.line))
	} else {
		f.semanticError(fmt.Sprintf("no visible label '%s' for <goto> at line %d", g.name, g.line))
	}
}

func (f *function) localVariable(i int) *localVariable {
	index := f.p.activeVariables[f.firstLocal+i]
	return &f.f.localVariables[index]
}

func (f *function) AdjustLocalVariables(n int) {
	for f.activeVariableCount += n; n != 0; n-- {
		f.localVariable(f.activeVariableCount - n).startPC = pc(f.pc)
	}
}

func (f *function) removeLocalVariables(level int) {
	for i := level; i < f.activeVariableCount; i++ {
		f.localVariable(i).endPC = pc(f.pc)
	}
	f.p.activeVariables = f.p.activeVariables[:len(f.p.activeVariables)-(f.activeVariableCount-level)]
	f.activeVariableCount = level
}

func (f *function) MakeLocalVariable(name string) {
	r := len(f.f.localVariables)
	f.f.localVariables = append(f.f.localVariables, localVariable{name: name})
	f.p.checkLimit(len(f.p.activeVariables)+1-f.firstLocal, maxLocalVariables, "local variables")
	f.p.activeVariables = append(f.p.activeVariables, r)
}

func (f *function) MakeGoto(name string, line, pc int) {
	f.p.pendingGotos = append(f.p.pendingGotos, label{name: name, line: line, pc: pc, activeVariableCount: f.activeVariableCount})
	f.findLabel(len(f.p.pendingGotos) - 1)
}

func (f *function) MakeLabel(name string, line int) int {
	f.p.activeLabels = append(f.p.activeLabels, label{name: name, line: line, pc: f.pc, activeVariableCount: f.activeVariableCount})
	return len(f.p.activeLabels) - 1
}

func (f *function) closeGoto(i int, l label) {
	g := f.p.activeLabels[i]
	if f.assert(g.name == l.name); g.activeVariableCount < l.activeVariableCount {
		f.semanticError(fmt.Sprintf("<goto %s> at line %d jumps into the scope of local '%s'", g.name, g.line, f.localVariable(g.activeVariableCount).name))
	}
	f.PatchList(g.pc, l.pc)
	copy(f.p.pendingGotos[i:], f.p.pendingGotos[i+1:])
	f.p.pendingGotos = f.p.pendingGotos[:len(f.p.pendingGotos)-1]
}

func (f *function) findLabel(i int) int {
	g, b := f.p.pendingGotos[i], f.block
	for _, l := range f.p.activeLabels[b.firstLabel:] {
		if l.name == g.name {
			if g.activeVariableCount > l.activeVariableCount && (b.hasUpValue || len(f.p.activeLabels) > b.firstLabel) {
				f.PatchClose(g.pc, l.activeVariableCount)
			}
			f.closeGoto(i, l)
			return 0
		}
	}
	return 1
}

func (f *function) CheckRepeatedLabel(name string) {
	for _, l := range f.p.activeLabels[f.block.firstLabel:] {
		if l.name == name {
			f.semanticError(fmt.Sprintf("label '%s' already defined on line %d", name, l.line))
		}
	}
}

func (f *function) FindGotos(label int) {
	for i, l := f.block.firstGoto, f.p.activeLabels[label]; i < len(f.p.pendingGotos); {
		if f.p.pendingGotos[i].name == l.name {
			f.closeGoto(i, l)
		} else {
			i++
		}
	}
}

func (f *function) moveGotosOut(b block) {
	for i := b.firstGoto; i < len(f.p.pendingGotos); i += f.findLabel(i) {
		if f.p.pendingGotos[i].activeVariableCount > b.activeVariableCount {
			if b.hasUpValue {
				f.PatchClose(f.p.pendingGotos[i].pc, b.activeVariableCount)
			}
			f.p.pendingGotos[i].activeVariableCount = b.activeVariableCount
		}
	}
}

func (f *function) breakLabel() {
	f.FindGotos(f.MakeLabel("break", 0))
}

func (f *function) LeaveBlock() {
	b := f.block
	if b.previous != nil && b.hasUpValue { // create a 'jump to here' to close upvalues
		j := f.Jump()
		f.PatchClose(j, b.activeVariableCount)
		f.PatchToHere(j)
	}
	if b.isLoop {
		f.breakLabel() // close pending breaks
	}
	f.block = b.previous
	f.removeLocalVariables(b.activeVariableCount)
	f.assert(b.activeVariableCount == f.activeVariableCount)
	f.freeRegisterCount = f.activeVariableCount
	f.p.activeLabels = f.p.activeLabels[:b.firstLabel]
	if b.previous != nil { // inner block
		f.moveGotosOut(*b) // update pending gotos to outer block
	} else if b.firstGoto < len(f.p.pendingGotos) { // pending gotos in outer block
		f.undefinedGotoError(f.p.pendingGotos[b.firstGoto])
	}
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func not(b int) int {
	if b == 0 {
		return 1
	}
	return 0
}

func makeExpression(kind, info int) exprDesc {
	return exprDesc{f: noJump, t: noJump, kind: kind, info: info}
}

func (f *function) semanticError(message string) {
	f.p.t = 0 // remove "near to" from final message
	f.p.syntaxError(message)
}

func (f *function) unreachable()                        { f.assert(false) }
func (f *function) assert(cond bool)                    { f.p.l.assert(cond) }
func (f *function) Instruction(e exprDesc) *instruction { return &f.f.code[e.info] }
func (e exprDesc) hasJumps() bool                       { return e.t != e.f }
func (e exprDesc) isNumeral() bool                      { return e.kind == kindNumber && e.t == noJump && e.f == noJump }
func (e exprDesc) isVariable() bool                     { return kindLocal <= e.kind && e.kind <= kindIndexed }
func (e exprDesc) hasMultipleReturns() bool             { return e.kind == kindCall || e.kind == kindVarArg }

func (f *function) Encode(i instruction) int {
	f.dischargeJumpPC()
	f.f.code = append(f.f.code, i) // TODO check that we always only append
	f.f.lineInfo = append(f.f.lineInfo, int32(f.p.lastLine))
	f.pc++
	return f.pc - 1
}

func (f *function) EncodeABC(op opCode, a, b, c int) int {
	f.assert(opMode(op) == iABC)
	f.assert(bMode(op) != opArgN || b == 0)
	f.assert(cMode(op) != opArgN || c == 0)
	f.assert(a <= maxArgA && b <= maxArgB && c <= maxArgC)
	return f.Encode(createABC(op, a, b, c))
}

func (f *function) EncodeABx(op opCode, a, bx int) int {
	f.assert(opMode(op) == iABx || opMode(op) == iAsBx)
	f.assert(cMode(op) == opArgN)
	f.assert(a <= maxArgA && bx <= maxArgBx)
	return f.Encode(createABx(op, a, bx))
}

func (f *function) EncodeAsBx(op opCode, a, sbx int) int {
	return f.EncodeABx(op, a, sbx+maxArgSBx)
}

func (f *function) encodeExtraArg(a int) int {
	f.assert(a <= maxArgAx)
	return f.Encode(createAx(opExtraArg, a))
}

func (f *function) EncodeConstant(r, constant int) int {
	if constant <= maxArgBx {
		return f.EncodeABx(opLoadConstant, r, constant)
	}
	pc := f.EncodeABx(opLoadConstant, r, 0)
	f.encodeExtraArg(constant)
	return pc
}

func (f *function) EncodeString(s string) exprDesc {
	return makeExpression(kindConstant, f.StringConstant(s))
}

func (f *function) LoadNil(from, n int) {
	if f.pc > f.lastTarget { // no jumps to current position
		if previous := &f.f.code[f.pc-1]; previous.opCode() == opLoadNil {
			if pf, pl, l := previous.a(), previous.a()+previous.b(), from+n-1; pf <= from && from < pl || from <= pf && pf < l { // can connect both
				from, l = min(from, pf), max(l, pl)
				previous.setA(from)
				previous.setB(l - from)
				return
			}
		}
	}
	f.EncodeABC(opLoadNil, from, n-1, 0)
}

func (f *function) Jump() int {
	jumpPC := f.jumpPC
	f.jumpPC = noJump
	return f.Concatenate(f.EncodeAsBx(opJump, 0, noJump), jumpPC)
}

func (f *function) JumpTo(target int) {
	f.PatchList(f.Jump(), target)
}

func (f *function) ReturnNone() {
	f.EncodeABC(opReturn, 0, 1, 0)
}

func (f *function) Return(e exprDesc, resultCount int) {
	if e.hasMultipleReturns() {
		if f.SetReturns(e, resultCount); e.kind == kindCall && resultCount == 1 {
			f.Instruction(e).setOpCode(opTailCall)
			f.assert(f.Instruction(e).a() == f.activeVariableCount)
		}
		f.EncodeABC(opReturn, f.activeVariableCount, MultipleReturns+1, 0)
	} else if resultCount == 1 {
		f.EncodeABC(opReturn, f.ExpressionToAnyRegister(e).info, 1+1, 0)
	} else {
		_ = f.ExpressionToNextRegister(e)
		f.assert(resultCount == f.freeRegisterCount-f.activeVariableCount)
		f.EncodeABC(opReturn, f.activeVariableCount, resultCount+1, 0)
	}
}

func (f *function) conditionalJump(op opCode, a, b, c int) int {
	f.EncodeABC(op, a, b, c)
	return f.Jump()
}

func (f *function) fixJump(pc, dest int) {
	f.assert(dest != noJump)
	offset := dest - (pc + 1)
	if abs(offset) > maxArgSBx {
		f.p.syntaxError("control structure too long")
	}
	f.f.code[pc].setSBx(offset)
}

func (f *function) Label() int {
	f.lastTarget = f.pc
	return f.pc
}

func (f *function) jump(pc int) int {
	if offset := f.f.code[pc].sbx(); offset != noJump {
		return pc + 1 + offset
	}
	return noJump
}

func (f *function) jumpControl(pc int) *instruction {
	if pc >= 1 && testTMode(f.f.code[pc-1].opCode()) {
		return &f.f.code[pc-1]
	}
	return &f.f.code[pc]
}

func (f *function) needValue(list int) bool {
	for ; list != noJump; list = f.jump(list) {
		if f.jumpControl(list).opCode() != opTestSet {
			return true
		}
	}
	return false
}

func (f *function) patchTestRegister(node, register int) bool {
	if i := f.jumpControl(node); i.opCode() != opTestSet {
		return false
	} else if register != noRegister && register != i.b() {
		i.setA(register)
	} else {
		*i = createABC(opTest, i.b(), 0, i.c())
	}
	return true
}

func (f *function) removeValues(list int) {
	for ; list != noJump; list = f.jump(list) {
		_ = f.patchTestRegister(list, noRegister)
	}
}

func (f *function) patchListHelper(list, target, register, defaultTarget int) {
	for list != noJump {
		next := f.jump(list)
		if f.patchTestRegister(list, register) {
			f.fixJump(list, target)
		} else {
			f.fixJump(list, defaultTarget)
		}
		list = next
	}
}

func (f *function) dischargeJumpPC() {
	f.patchListHelper(f.jumpPC, f.pc, noRegister, f.pc)
	f.jumpPC = noJump
}

func (f *function) PatchList(list, target int) {
	if target == f.pc {
		f.PatchToHere(list)
	} else {
		f.assert(target < f.pc)
		f.patchListHelper(list, target, noRegister, target)
	}
}

func (f *function) PatchClose(list, level int) {
	for level, next := level+1, 0; list != noJump; list = next {
		next = f.jump(list)
		f.assert(f.f.code[list].opCode() == opJump && f.f.code[list].a() == 0 || f.f.code[list].a() >= level)
		f.f.code[list].setA(level)
	}
}

func (f *function) PatchToHere(list int) {
	f.Label()
	f.jumpPC = f.Concatenate(f.jumpPC, list)
}

func (f *function) Concatenate(l1, l2 int) int {
	switch {
	case l2 == noJump:
	case l1 == noJump:
		return l2
	default:
		list := l1
		for next := f.jump(list); next != noJump; list, next = next, f.jump(next) {
		}
		f.fixJump(list, l2)
	}
	return l1
}

func (f *function) addConstant(k, v value) (index int) {
	if old, ok := f.h.at(k).(float64); ok {
		if index = int(old); f.f.constants[index] == v {
			return
		}
	}
	index = len(f.f.constants)
	f.h.put(k, float64(index))
	f.f.constants = append(f.f.constants, v)
	return
}

func (f *function) NumberConstant(n float64) int {
	if n == 0.0 && math.Signbit(n) {
		return f.addConstant("-0.0", n)
	} else if n == 0.0 {
		return f.addConstant("0.0", n)
	} else if math.IsNaN(n) {
		return f.addConstant("NaN", n)
	}
	return f.addConstant(n, n)
}

func (f *function) CheckStack(n int) {
	if n += f.freeRegisterCount; n >= maxStack {
		f.p.syntaxError("function or expression too complex")
	} else if n > f.f.maxStackSize {
		f.f.maxStackSize = n
	}
}

func (f *function) ReserveRegisters(n int) {
	f.CheckStack(n)
	f.freeRegisterCount += n
}

func (f *function) freeRegister(r int) {
	if !isConstant(r) && r >= f.activeVariableCount {
		f.freeRegisterCount--
		f.assert(r == f.freeRegisterCount)
	}
}

func (f *function) freeExpression(e exprDesc) {
	if e.kind == kindNonRelocatable {
		f.freeRegister(e.info)
	}
}

func (f *function) StringConstant(s string) int { return f.addConstant(s, s) }
func (f *function) booleanConstant(b bool) int  { return f.addConstant(b, b) }
func (f *function) nilConstant() int            { return f.addConstant(f.h, nil) }

func (f *function) SetReturns(e exprDesc, resultCount int) {
	if e.kind == kindCall {
		f.Instruction(e).setC(resultCount + 1)
	} else if e.kind == kindVarArg {
		f.Instruction(e).setB(resultCount + 1)
		f.Instruction(e).setA(f.freeRegisterCount)
		f.ReserveRegisters(1)
	}
}

func (f *function) SetReturn(e exprDesc) exprDesc {
	if e.kind == kindCall {
		e.kind, e.info = kindNonRelocatable, f.Instruction(e).a()
	} else if e.kind == kindVarArg {
		f.Instruction(e).setB(2)
		e.kind = kindRelocatable
	}
	return e
}

func (f *function) DischargeVariables(e exprDesc) exprDesc {
	switch e.kind {
	case kindLocal:
		e.kind = kindNonRelocatable
	case kindUpValue:
		e.kind, e.info = kindRelocatable, f.EncodeABC(opGetUpValue, 0, e.info, 0)
	case kindIndexed:
		if f.freeRegister(e.index); e.tableType == kindLocal {
			f.freeRegister(e.table)
			e.kind, e.info = kindRelocatable, f.EncodeABC(opGetTable, 0, e.table, e.index)
		} else {
			e.kind, e.info = kindRelocatable, f.EncodeABC(opGetTableUp, 0, e.table, e.index)
		}
	case kindVarArg, kindCall:
		e = f.SetReturn(e)
	}
	return e
}

func (f *function) dischargeToRegister(e exprDesc, r int) exprDesc {
	switch e = f.DischargeVariables(e); e.kind {
	case kindNil:
		f.LoadNil(r, 1)
	case kindFalse:
		f.EncodeABC(opLoadBool, r, 0, 0)
	case kindTrue:
		f.EncodeABC(opLoadBool, r, 1, 0)
	case kindConstant:
		f.EncodeConstant(r, e.info)
	case kindNumber:
		f.EncodeConstant(r, f.NumberConstant(e.value))
	case kindRelocatable:
		f.Instruction(e).setA(r)
	case kindNonRelocatable:
		if r != e.info {
			f.EncodeABC(opMove, r, e.info, 0)
		}
	default:
		f.assert(e.kind == kindVoid || e.kind == kindJump)
		return e
	}
	e.kind, e.info = kindNonRelocatable, r
	return e
}

func (f *function) dischargeToAnyRegister(e exprDesc) exprDesc {
	if e.kind != kindNonRelocatable {
		f.ReserveRegisters(1)
		e = f.dischargeToRegister(e, f.freeRegisterCount-1)
	}
	return e
}

func (f *function) encodeLabel(a, b, jump int) int {
	f.Label()
	return f.EncodeABC(opLoadBool, a, b, jump)
}

func (f *function) expressionToRegister(e exprDesc, r int) exprDesc {
	if e = f.dischargeToRegister(e, r); e.kind == kindJump {
		e.t = f.Concatenate(e.t, e.info)
	}
	if e.hasJumps() {
		loadFalse, loadTrue := noJump, noJump
		if f.needValue(e.t) || f.needValue(e.f) {
			jump := noJump
			if e.kind != kindJump {
				jump = f.Jump()
			}
			loadFalse, loadTrue = f.encodeLabel(r, 0, 1), f.encodeLabel(r, 1, 0)
			f.PatchToHere(jump)
		}
		end := f.Label()
		f.patchListHelper(e.f, end, r, loadFalse)
		f.patchListHelper(e.t, end, r, loadTrue)
	}
	e.f, e.t, e.info, e.kind = noJump, noJump, r, kindNonRelocatable
	return e
}

func (f *function) ExpressionToNextRegister(e exprDesc) exprDesc {
	e = f.DischargeVariables(e)
	f.freeExpression(e)
	f.ReserveRegisters(1)
	return f.expressionToRegister(e, f.freeRegisterCount-1)
}

func (f *function) ExpressionToAnyRegister(e exprDesc) exprDesc {
	if e = f.DischargeVariables(e); e.kind == kindNonRelocatable {
		if !e.hasJumps() {
			return e
		}
		if e.info >= f.activeVariableCount {
			return f.expressionToRegister(e, e.info)
		}
	}
	return f.ExpressionToNextRegister(e)
}

func (f *function) ExpressionToAnyRegisterOrUpValue(e exprDesc) exprDesc {
	if e.kind != kindUpValue || e.hasJumps() {
		e = f.ExpressionToAnyRegister(e)
	}
	return e
}

func (f *function) ExpressionToValue(e exprDesc) exprDesc {
	if e.hasJumps() {
		return f.ExpressionToAnyRegister(e)
	}
	return f.DischargeVariables(e)
}

func (f *function) ExpressionToRegisterOrConstant(e exprDesc) (exprDesc, int) {
	switch e = f.ExpressionToValue(e); e.kind {
	case kindTrue, kindFalse:
		if len(f.f.constants) <= maxIndexRK {
			e.info, e.kind = f.booleanConstant(e.kind == kindTrue), kindConstant
			return e, asConstant(e.info)
		}
	case kindNil:
		if len(f.f.constants) <= maxIndexRK {
			e.info, e.kind = f.nilConstant(), kindConstant
			return e, asConstant(e.info)
		}
	case kindNumber:
		e.info, e.kind = f.NumberConstant(e.value), kindConstant
		fallthrough
	case kindConstant:
		if e.info <= maxIndexRK {
			return e, asConstant(e.info)
		}
	}
	e = f.ExpressionToAnyRegister(e)
	return e, e.info
}

func (f *function) StoreVariable(v, e exprDesc) {
	switch v.kind {
	case kindLocal:
		f.freeExpression(e)
		f.expressionToRegister(e, v.info)
		return
	case kindUpValue:
		e = f.ExpressionToAnyRegister(e)
		f.EncodeABC(opSetUpValue, e.info, v.info, 0)
	case kindIndexed:
		var r int
		e, r = f.ExpressionToRegisterOrConstant(e)
		if v.tableType == kindLocal {
			f.EncodeABC(opSetTable, v.table, v.index, r)
		} else {
			f.EncodeABC(opSetTableUp, v.table, v.index, r)
		}
	default:
		f.unreachable()
	}
	f.freeExpression(e)
}

func (f *function) Self(e, key exprDesc) exprDesc {
	e = f.ExpressionToAnyRegister(e)
	r := e.info
	f.freeExpression(e)
	result := exprDesc{info: f.freeRegisterCount, kind: kindNonRelocatable} // base register for opSelf
	f.ReserveRegisters(2)                                                   // function and 'self' produced by opSelf
	key, k := f.ExpressionToRegisterOrConstant(key)
	f.EncodeABC(opSelf, result.info, r, k)
	f.freeExpression(key)
	return result
}

func (f *function) invertJump(pc int) {
	i := f.jumpControl(pc)
	f.p.l.assert(testTMode(i.opCode()) && i.opCode() != opTestSet && i.opCode() != opTest)
	i.setA(not(i.a()))
}

func (f *function) jumpOnCondition(e exprDesc, cond int) int {
	if e.kind == kindRelocatable {
		if i := f.Instruction(e); i.opCode() == opNot {
			f.pc-- // remove previous opNot
			return f.conditionalJump(opTest, i.b(), 0, not(cond))
		}
	}
	e = f.dischargeToAnyRegister(e)
	f.freeExpression(e)
	return f.conditionalJump(opTestSet, noRegister, e.info, cond)
}

func (f *function) GoIfTrue(e exprDesc) exprDesc {
	pc := noJump
	switch e = f.DischargeVariables(e); e.kind {
	case kindJump:
		f.invertJump(e.info)
		pc = e.info
	case kindConstant, kindNumber, kindTrue:
	default:
		pc = f.jumpOnCondition(e, 0)
	}
	e.f = f.Concatenate(e.f, pc)
	f.PatchToHere(e.t)
	e.t = noJump
	return e
}

func (f *function) GoIfFalse(e exprDesc) exprDesc {
	pc := noJump
	switch e = f.DischargeVariables(e); e.kind {
	case kindJump:
		pc = e.info
	case kindNil, kindFalse:
	default:
		pc = f.jumpOnCondition(e, 1)
	}
	e.t = f.Concatenate(e.t, pc)
	f.PatchToHere(e.f)
	e.f = noJump
	return e
}

func (f *function) encodeNot(e exprDesc) exprDesc {
	switch e = f.DischargeVariables(e); e.kind {
	case kindNil, kindFalse:
		e.kind = kindTrue
	case kindConstant, kindNumber, kindTrue:
		e.kind = kindFalse
	case kindJump:
		f.invertJump(e.info)
	case kindRelocatable, kindNonRelocatable:
		e = f.dischargeToAnyRegister(e)
		f.freeExpression(e)
		e.info, e.kind = f.EncodeABC(opNot, 0, e.info, 0), kindRelocatable
	default:
		f.unreachable()
	}
	e.f, e.t = e.t, e.f
	f.removeValues(e.f)
	f.removeValues(e.t)
	return e
}

func (f *function) Indexed(t, k exprDesc) (r exprDesc) {
	f.assert(!t.hasJumps())
	r.table = t.info
	k, r.index = f.ExpressionToRegisterOrConstant(k)
	if t.kind == kindUpValue {
		r.tableType = kindUpValue
	} else {
		f.assert(t.kind == kindNonRelocatable || t.kind == kindLocal)
		r.tableType = kindLocal
	}
	r.kind = kindIndexed
	return
}

func foldConstants(op opCode, e1, e2 exprDesc) (exprDesc, bool) {
	if !e1.isNumeral() || !e2.isNumeral() {
		return e1, false
	} else if (op == opDiv || op == opMod) && e2.value == 0.0 {
		return e1, false
	}
	e1.value = arith(int(op-opAdd)+OpAdd, e1.value, e2.value)
	return e1, true
}

func (f *function) encodeArithmetic(op opCode, e1, e2 exprDesc, line int) exprDesc {
	if e, folded := foldConstants(op, e1, e2); folded {
		return e
	}
	o2 := 0
	if op != opUnaryMinus && op != opLength {
		e2, o2 = f.ExpressionToRegisterOrConstant(e2)
	}
	e1, o1 := f.ExpressionToRegisterOrConstant(e1)
	if o1 > o2 {
		f.freeExpression(e1)
		f.freeExpression(e2)
	} else {
		f.freeExpression(e2)
		f.freeExpression(e1)
	}
	e1.info, e1.kind = f.EncodeABC(op, 0, o1, o2), kindRelocatable
	f.FixLine(line)
	return e1
}

func (f *function) Prefix(op int, e exprDesc, line int) exprDesc {
	switch op {
	case oprMinus:
		if e.isNumeral() {
			e.value = -e.value
			return e
		} else {
			return f.encodeArithmetic(opUnaryMinus, f.ExpressionToAnyRegister(e), makeExpression(kindNumber, 0), line)
		}
	case oprNot:
		return f.encodeNot(e)
	case oprLength:
		return f.encodeArithmetic(opLength, f.ExpressionToAnyRegister(e), makeExpression(kindNumber, 0), line)
	}
	panic("unreachable")
}

func (f *function) Infix(op int, e exprDesc) exprDesc {
	switch op {
	case oprAnd:
		e = f.GoIfTrue(e)
	case oprOr:
		e = f.GoIfFalse(e)
	case oprConcat:
		e = f.ExpressionToNextRegister(e)
	case oprAdd, oprSub, oprMul, oprDiv, oprMod, oprPow:
		if !e.isNumeral() {
			e, _ = f.ExpressionToRegisterOrConstant(e)
		}
	default:
		e, _ = f.ExpressionToRegisterOrConstant(e)
	}
	return e
}

func (f *function) encodeComparison(op opCode, cond int, e1, e2 exprDesc) exprDesc {
	e1, o1 := f.ExpressionToRegisterOrConstant(e1)
	e2, o2 := f.ExpressionToRegisterOrConstant(e2)
	f.freeExpression(e2)
	f.freeExpression(e1)
	if cond == 0 && op != opEqual {
		o1, o2, cond = o2, o1, 1
	}
	return makeExpression(kindJump, f.conditionalJump(op, cond, o1, o2))
}

func (f *function) Postfix(op int, e1, e2 exprDesc, line int) exprDesc {
	switch op {
	case oprAnd:
		f.assert(e1.t == noJump)
		e2 = f.DischargeVariables(e2)
		e2.f = f.Concatenate(e2.f, e1.f)
		return e2
	case oprOr:
		f.assert(e1.f == noJump)
		e2 = f.DischargeVariables(e2)
		e2.t = f.Concatenate(e2.t, e1.t)
		return e2
	case oprConcat:
	case oprAdd, oprSub, oprMul, oprDiv, oprMod, oprPow:
		return f.encodeArithmetic(opCode(op-oprAdd)+opAdd, e1, e2, line)
	case oprEq, oprLT, oprLE:
		return f.encodeComparison(opCode(op-oprEq)+opEqual, 1, e1, e2)
	case oprNE, oprGT, oprGE:
		return f.encodeComparison(opCode(op-oprNE)+opEqual, 0, e1, e2)
	}
	panic("unreachable")
}

func (f *function) FixLine(line int) {
	f.f.lineInfo[f.pc-1] = int32(line)
}

func (f *function) SetList(base, elementCount, storeCount int) {
	if f.assert(storeCount != 0); storeCount == MultipleReturns {
		storeCount = 0
	}
	if c := (elementCount-1)/listItemsPerFlush + 1; c <= maxArgC {
		f.EncodeABC(opSetList, base, storeCount, c)
	} else if c <= maxArgAx {
		f.EncodeABC(opSetList, base, storeCount, 0)
		f.encodeExtraArg(c)
	} else {
		f.p.syntaxError("constructor too long")
	}
	f.freeRegisterCount = base + 1
}

func (f *function) CheckConflict(t *assignmentTarget, e exprDesc) {
	extra, conflict := f.freeRegisterCount, false
	for ; t != nil; t = t.previous {
		if t.kind == kindIndexed {
			if t.tableType == e.kind && t.table == e.info {
				conflict = true
				t.table, t.tableType = extra, kindLocal
			}
			if e.kind == kindLocal && t.index == e.info {
				conflict = true
				t.index = extra
			}
		}
	}
	if conflict {
		if e.kind == kindLocal {
			f.EncodeABC(opMove, extra, e.info, 0)
		} else {
			f.EncodeABC(opGetUpValue, extra, e.info, 0)
		}
		f.ReserveRegisters(1)
	}
}

func (f *function) AdjustAssignment(variableCount, expressionCount int, e exprDesc) {
	if extra := variableCount - expressionCount; e.hasMultipleReturns() {
		if extra++; extra < 0 {
			extra = 0
		}
		if f.SetReturns(e, extra); extra > 1 {
			f.ReserveRegisters(extra - 1)
		}
	} else {
		if expressionCount > 0 {
			_ = f.ExpressionToNextRegister(e)
		}
		if extra > 0 {
			r := f.freeRegisterCount
			f.ReserveRegisters(extra)
			f.LoadNil(r, extra)
		}
	}
}

func (f *function) makeUpValue(name string, e exprDesc) int {
	f.p.checkLimit(len(f.f.upValues)+1, maxUpValue, "upvalues")
	f.f.upValues = append(f.f.upValues, upValueDesc{name: name, isLocal: e.kind == kindLocal, index: e.info})
	return len(f.f.upValues) - 1
}

func singleVariableHelper(f *function, name string, base bool) (e exprDesc, found bool) {
	owningBlock := func(b *block, level int) *block {
		for b.activeVariableCount > level {
			b = b.previous
		}
		return b
	}
	find := func() (int, bool) {
		for i := f.activeVariableCount - 1; i >= 0; i-- {
			if name == f.localVariable(i).name {
				return i, true
			}
		}
		return 0, false
	}
	findUpValue := func() (int, bool) {
		for i, u := range f.f.upValues {
			if u.name == name {
				return i, true
			}
		}
		return 0, false
	}
	if f == nil {
		return
	}
	var v int
	if v, found = find(); found {
		if e = makeExpression(kindLocal, v); !base {
			owningBlock(f.block, v).hasUpValue = true
		}
		return
	}
	if v, found = findUpValue(); found {
		return makeExpression(kindUpValue, v), true
	}
	if e, found = singleVariableHelper(f.previous, name, false); !found {
		return
	}
	return makeExpression(kindUpValue, f.makeUpValue(name, e)), true
}

func (f *function) SingleVariable(name string) (e exprDesc) {
	var found bool
	if e, found = singleVariableHelper(f, name, true); !found {
		e, found = singleVariableHelper(f, "_ENV", true)
		f.assert(found && (e.kind == kindLocal || e.kind == kindUpValue))
		e = f.Indexed(e, f.EncodeString(name))
	}
	return
}

func (f *function) OpenConstructor() (pc int, t exprDesc) {
	pc = f.EncodeABC(opNewTable, 0, 0, 0)
	t = f.ExpressionToNextRegister(makeExpression(kindRelocatable, pc))
	return
}

func (f *function) FlushFieldToConstructor(tableRegister, freeRegisterCount int, k exprDesc, v func() exprDesc) {
	_, rk := f.ExpressionToRegisterOrConstant(k)
	_, rv := f.ExpressionToRegisterOrConstant(v())
	f.EncodeABC(opSetTable, tableRegister, rk, rv)
	f.freeRegisterCount = freeRegisterCount
}

func (f *function) FlushToConstructor(tableRegister, pending, arrayCount int, e exprDesc) int {
	f.ExpressionToNextRegister(e)
	if pending == listItemsPerFlush {
		f.SetList(tableRegister, arrayCount, listItemsPerFlush)
		pending = 0
	}
	return pending
}

func (f *function) CloseConstructor(pc, tableRegister, pending, arrayCount, hashCount int, e exprDesc) {
	if pending != 0 {
		if e.hasMultipleReturns() {
			f.SetReturns(e, MultipleReturns)
			f.SetList(tableRegister, arrayCount, MultipleReturns)
			arrayCount--
		} else {
			if e.kind != kindVoid {
				f.ExpressionToNextRegister(e)
			}
			f.SetList(tableRegister, arrayCount, pending)
		}
	}
	f.f.code[pc].setB(int(float8FromInt(arrayCount)))
	f.f.code[pc].setC(int(float8FromInt(hashCount)))
}
