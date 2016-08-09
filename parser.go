package lua

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type parser struct {
	scanner
	function                   *function
	activeVariables            []int
	pendingGotos, activeLabels []label
}

func (p *parser) checkCondition(c bool, message string) {
	if !c {
		p.syntaxError(message)
	}
}

func (p *parser) checkName() string {
	p.check(tkName)
	s := p.s
	p.next()
	return s
}

func (p *parser) checkLimit(val, limit int, what string) {
	if val > limit {
		where := "main function"
		if line := p.function.f.lineDefined; line != 0 {
			where = fmt.Sprintf("function at line %d", line)
		}
		p.syntaxError(fmt.Sprintf("too many %s (limit is %d) in %s", what, limit, where))
	}
}

func (p *parser) checkNext(t rune) {
	p.check(t)
	p.next()
}

func (p *parser) checkNameAsExpression() exprDesc { return p.function.EncodeString(p.checkName()) }
func (p *parser) singleVariable() exprDesc        { return p.function.SingleVariable(p.checkName()) }
func (p *parser) leaveLevel()                     { p.l.nestedGoCallCount-- }
func (p *parser) enterLevel() {
	p.l.nestedGoCallCount++
	p.checkLimit(p.l.nestedGoCallCount, maxCallCount, "Go levels")
}

func (p *parser) expressionList() (e exprDesc, n int) {
	for n, e = 1, p.expression(); p.testNext(','); n, e = n+1, p.expression() {
		_ = p.function.ExpressionToNextRegister(e)
	}
	return
}

func (p *parser) field(tableRegister, a, h, pending int, e exprDesc) (int, int, int, exprDesc) {
	freeRegisterCount := p.function.freeRegisterCount
	hashField := func(k exprDesc) {
		h++
		p.checkNext('=')
		p.function.FlushFieldToConstructor(tableRegister, freeRegisterCount, k, p.expression)
	}
	switch {
	case p.t == tkName && p.lookAhead() == '=':
		p.checkLimit(h, maxInt, "items in a constructor")
		hashField(p.checkNameAsExpression())
	case p.t == '[':
		hashField(p.index())
	default:
		e = p.expression()
		p.checkLimit(a, maxInt, "items in a constructor")
		a++
		pending++
	}
	return a, h, pending, e
}

func (p *parser) constructor() exprDesc {
	pc, t := p.function.OpenConstructor()
	line, a, h, pending := p.lineNumber, 0, 0, 0
	var e exprDesc
	if p.checkNext('{'); p.t != '}' {
		for a, h, pending, e = p.field(t.info, a, h, pending, e); (p.testNext(',') || p.testNext(';')) && p.t != '}'; {
			if e.kind != kindVoid {
				pending = p.function.FlushToConstructor(t.info, pending, a, e)
				e.kind = kindVoid
			}
			a, h, pending, e = p.field(t.info, a, h, pending, e)
		}
	}
	p.checkMatch('}', '{', line)
	p.function.CloseConstructor(pc, t.info, pending, a, h, e)
	return t
}

func (p *parser) functionArguments(f exprDesc, line int) exprDesc {
	var args exprDesc
	switch p.t {
	case '(':
		p.next()
		if p.t == ')' {
			args.kind = kindVoid
		} else {
			args, _ = p.expressionList()
			p.function.SetMultipleReturns(args)
		}
		p.checkMatch(')', '(', line)
	case '{':
		args = p.constructor()
	case tkString:
		args = p.function.EncodeString(p.s)
		p.next()
	default:
		p.syntaxError("function arguments expected")
	}
	base, parameterCount := f.info, MultipleReturns
	if !args.hasMultipleReturns() {
		if args.kind != kindVoid {
			args = p.function.ExpressionToNextRegister(args)
		}
		parameterCount = p.function.freeRegisterCount - (base + 1)
	}
	e := makeExpression(kindCall, p.function.EncodeABC(opCall, base, parameterCount+1, 2))
	p.function.FixLine(line)
	p.function.freeRegisterCount = base + 1 // call removed function and args & leaves (unless changed) one result
	return e
}

func (p *parser) primaryExpression() (e exprDesc) {
	switch p.t {
	case '(':
		line := p.lineNumber
		p.next()
		e = p.expression()
		p.checkMatch(')', '(', line)
		e = p.function.DischargeVariables(e)
	case tkName:
		e = p.singleVariable()
	default:
		p.syntaxError("unexpected symbol")
	}
	return
}

func (p *parser) suffixedExpression() exprDesc {
	line := p.lineNumber
	e := p.primaryExpression()
	for {
		switch p.t {
		case '.':
			e = p.fieldSelector(e)
		case '[':
			e = p.function.Indexed(p.function.ExpressionToAnyRegisterOrUpValue(e), p.index())
		case ':':
			p.next()
			e = p.functionArguments(p.function.Self(e, p.checkNameAsExpression()), line)
		case '(', tkString, '{':
			e = p.functionArguments(p.function.ExpressionToNextRegister(e), line)
		default:
			return e
		}
	}
	panic("unreachable")
}

func (p *parser) simpleExpression() (e exprDesc) {
	switch p.t {
	case tkNumber:
		e = makeExpression(kindNumber, 0)
		e.value = p.n
	case tkString:
		e = p.function.EncodeString(p.s)
	case tkNil:
		e = makeExpression(kindNil, 0)
	case tkTrue:
		e = makeExpression(kindTrue, 0)
	case tkFalse:
		e = makeExpression(kindFalse, 0)
	case tkDots:
		p.checkCondition(p.function.f.isVarArg, "cannot use '...' outside a vararg function")
		e = makeExpression(kindVarArg, p.function.EncodeABC(opVarArg, 0, 1, 0))
	case '{':
		e = p.constructor()
		return
	case tkFunction:
		p.next()
		e = p.body(false, p.lineNumber)
		return
	default:
		e = p.suffixedExpression()
		return
	}
	p.next()
	return
}

func unaryOp(op rune) int {
	switch op {
	case tkNot:
		return oprNot
	case '-':
		return oprMinus
	case '#':
		return oprLength
	}
	return oprNoUnary
}

func binaryOp(op rune) int {
	switch op {
	case '+':
		return oprAdd
	case '-':
		return oprSub
	case '*':
		return oprMul
	case '/':
		return oprDiv
	case '%':
		return oprMod
	case '^':
		return oprPow
	case tkConcat:
		return oprConcat
	case tkNE:
		return oprNE
	case tkEq:
		return oprEq
	case '<':
		return oprLT
	case tkLE:
		return oprLE
	case '>':
		return oprGT
	case tkGE:
		return oprGE
	case tkAnd:
		return oprAnd
	case tkOr:
		return oprOr
	}
	return oprNoBinary
}

var priority []struct{ left, right int } = []struct{ left, right int }{
	{6, 6}, {6, 6}, {7, 7}, {7, 7}, {7, 7}, // `+' `-' `*' `/' `%'
	{10, 9}, {5, 4}, // ^, .. (right associative)
	{3, 3}, {3, 3}, {3, 3}, // ==, <, <=
	{3, 3}, {3, 3}, {3, 3}, // ~=, >, >=
	{2, 2}, {1, 1}, // and, or
}

const unaryPriority = 8

func (p *parser) subExpression(limit int) (e exprDesc, op int) {
	p.enterLevel()
	if u := unaryOp(p.t); u != oprNoUnary {
		line := p.lineNumber
		p.next()
		e, _ = p.subExpression(unaryPriority)
		e = p.function.Prefix(u, e, line)
	} else {
		e = p.simpleExpression()
	}
	op = binaryOp(p.t)
	for op != oprNoBinary && priority[op].left > limit {
		line := p.lineNumber
		p.next()
		e = p.function.Infix(op, e)
		e2, next := p.subExpression(priority[op].right)
		e = p.function.Postfix(op, e, e2, line)
		op = next
	}
	p.leaveLevel()
	return
}

func (p *parser) expression() (e exprDesc) {
	e, _ = p.subExpression(0)
	return
}

func (p *parser) blockFollow(withUntil bool) bool {
	switch p.t {
	case tkElse, tkElseif, tkEnd, tkEOS:
		return true
	case tkUntil:
		return withUntil
	}
	return false
}

func (p *parser) statementList() {
	for !p.blockFollow(true) {
		if p.t == tkReturn {
			p.statement()
			return
		}
		p.statement()
	}
}

func (p *parser) fieldSelector(e exprDesc) exprDesc {
	e = p.function.ExpressionToAnyRegisterOrUpValue(e)
	p.next() // skip dot or colon
	return p.function.Indexed(e, p.checkNameAsExpression())
}

func (p *parser) index() exprDesc {
	p.next() // skip '['
	e := p.function.ExpressionToValue(p.expression())
	p.checkNext(']')
	return e
}

func (p *parser) assignment(t *assignmentTarget, variableCount int) {
	if p.checkCondition(t.isVariable(), "syntax error"); p.testNext(',') {
		e := p.suffixedExpression()
		if e.kind != kindIndexed {
			p.function.CheckConflict(t, e)
		}
		p.checkLimit(variableCount+p.l.nestedGoCallCount, maxCallCount, "Go levels")
		p.assignment(&assignmentTarget{previous: t, exprDesc: e}, variableCount+1)
	} else {
		p.checkNext('=')
		if e, n := p.expressionList(); n != variableCount {
			if p.function.AdjustAssignment(variableCount, n, e); n > variableCount {
				p.function.freeRegisterCount -= n - variableCount // remove extra values
			}
		} else {
			p.function.StoreVariable(t.exprDesc, p.function.SetReturn(e))
			return // avoid default
		}
	}
	p.function.StoreVariable(t.exprDesc, makeExpression(kindNonRelocatable, p.function.freeRegisterCount-1))
}

func (p *parser) forBody(base, line, n int, isNumeric bool) {
	p.function.AdjustLocalVariables(3)
	p.checkNext(tkDo)
	prep := p.function.OpenForBody(base, n, isNumeric)
	p.block()
	p.function.CloseForBody(prep, base, line, n, isNumeric)
}

func (p *parser) forNumeric(name string, line int) {
	expr := func() { p.assert(p.function.ExpressionToNextRegister(p.expression()).kind == kindNonRelocatable) }
	base := p.function.freeRegisterCount
	p.function.MakeLocalVariable("(for index)")
	p.function.MakeLocalVariable("(for limit)")
	p.function.MakeLocalVariable("(for step)")
	p.function.MakeLocalVariable(name)
	p.checkNext('=')
	expr()
	p.checkNext(',')
	expr()
	if p.testNext(',') {
		expr()
	} else {
		p.function.EncodeConstant(p.function.freeRegisterCount, p.function.NumberConstant(1))
		p.function.ReserveRegisters(1)
	}
	p.forBody(base, line, 1, true)
}

func (p *parser) forList(name string) {
	n, base := 4, p.function.freeRegisterCount
	p.function.MakeLocalVariable("(for generator)")
	p.function.MakeLocalVariable("(for state)")
	p.function.MakeLocalVariable("(for control)")
	p.function.MakeLocalVariable(name)
	for ; p.testNext(','); n++ {
		p.function.MakeLocalVariable(p.checkName())
	}
	p.checkNext(tkIn)
	line := p.lineNumber
	e, c := p.expressionList()
	p.function.AdjustAssignment(3, c, e)
	p.function.CheckStack(3)
	p.forBody(base, line, n-3, false)
}

func (p *parser) forStatement(line int) {
	p.function.EnterBlock(true)
	p.next()
	switch name := p.checkName(); p.t {
	case '=':
		p.forNumeric(name, line)
	case ',', tkIn:
		p.forList(name)
	default:
		p.syntaxError("'=' or 'in' expected")
	}
	p.checkMatch(tkEnd, tkFor, line)
	p.function.LeaveBlock()
}

func (p *parser) testThenBlock(escapes int) int {
	var jumpFalse int
	p.next()
	e := p.expression()
	p.checkNext(tkThen)
	if p.t == tkGoto || p.t == tkBreak {
		e = p.function.GoIfFalse(e)
		p.function.EnterBlock(false)
		p.gotoStatement(e.t)
		p.skipEmptyStatements()
		if p.blockFollow(false) {
			p.function.LeaveBlock()
			return escapes
		}
		jumpFalse = p.function.Jump()
	} else {
		e = p.function.GoIfTrue(e)
		p.function.EnterBlock(false)
		jumpFalse = e.f
	}
	p.statementList()
	p.function.LeaveBlock()
	if p.t == tkElse || p.t == tkElseif {
		escapes = p.function.Concatenate(escapes, p.function.Jump())
	}
	p.function.PatchToHere(jumpFalse)
	return escapes
}

func (p *parser) ifStatement(line int) {
	escapes := p.testThenBlock(noJump)
	for p.t == tkElseif {
		escapes = p.testThenBlock(escapes)
	}
	if p.testNext(tkElse) {
		p.block()
	}
	p.checkMatch(tkEnd, tkIf, line)
	p.function.PatchToHere(escapes)
}

func (p *parser) block() {
	p.function.EnterBlock(false)
	p.statementList()
	p.function.LeaveBlock()
}

func (p *parser) whileStatement(line int) {
	p.next()
	top, conditionExit := p.function.Label(), p.condition()
	p.function.EnterBlock(true)
	p.checkNext(tkDo)
	p.block()
	p.function.JumpTo(top)
	p.checkMatch(tkEnd, tkWhile, line)
	p.function.LeaveBlock()
	p.function.PatchToHere(conditionExit) // false conditions finish the loop
}

func (p *parser) repeatStatement(line int) {
	top := p.function.Label()
	p.function.EnterBlock(true)  // loop block
	p.function.EnterBlock(false) // scope block
	p.next()
	p.statementList()
	p.checkMatch(tkUntil, tkRepeat, line)
	conditionExit := p.condition()
	if p.function.block.hasUpValue {
		p.function.PatchClose(conditionExit, p.function.block.activeVariableCount)
	}
	p.function.LeaveBlock()                  // finish scope
	p.function.PatchList(conditionExit, top) // close loop
	p.function.LeaveBlock()                  // finish loop
}

func (p *parser) condition() int {
	e := p.expression()
	if e.kind == kindNil {
		e.kind = kindFalse
	}
	return p.function.GoIfTrue(e).f
}

func (p *parser) gotoStatement(pc int) {
	if line := p.lineNumber; p.testNext(tkGoto) {
		p.function.MakeGoto(p.checkName(), line, pc)
	} else {
		p.next()
		p.function.MakeGoto("break", line, pc)
	}
}

func (p *parser) skipEmptyStatements() {
	for p.t == ';' || p.t == tkDoubleColon {
		p.statement()
	}
}

func (p *parser) labelStatement(label string, line int) {
	p.function.CheckRepeatedLabel(label)
	p.checkNext(tkDoubleColon)
	l := p.function.MakeLabel(label, line)
	p.skipEmptyStatements()
	if p.blockFollow(false) {
		p.activeLabels[l].activeVariableCount = p.function.block.activeVariableCount
	}
	p.function.FindGotos(l)
}

func (p *parser) parameterList() {
	n, isVarArg := 0, false
	if p.t != ')' {
		for first := true; first || (!isVarArg && p.testNext(',')); first = false {
			switch p.t {
			case tkName:
				p.function.MakeLocalVariable(p.checkName())
				n++
			case tkDots:
				p.next()
				isVarArg = true
			default:
				p.syntaxError("<name> or '...' expected")
			}
		}
	}
	// TODO the following lines belong in a *function method
	p.function.f.isVarArg = isVarArg
	p.function.AdjustLocalVariables(n)
	p.function.f.parameterCount = p.function.activeVariableCount
	p.function.ReserveRegisters(p.function.activeVariableCount)
}

func (p *parser) body(isMethod bool, line int) exprDesc {
	p.function.OpenFunction(line)
	p.checkNext('(')
	if isMethod {
		p.function.MakeLocalVariable("self")
		p.function.AdjustLocalVariables(1)
	}
	p.parameterList()
	p.checkNext(')')
	p.statementList()
	p.function.f.lastLineDefined = p.lineNumber
	p.checkMatch(tkEnd, tkFunction, line)
	return p.function.CloseFunction()
}

func (p *parser) functionName() (e exprDesc, isMethod bool) {
	for e = p.singleVariable(); p.t == '.'; e = p.fieldSelector(e) {
	}
	if p.t == ':' {
		e, isMethod = p.fieldSelector(e), true
	}
	return
}

func (p *parser) functionStatement(line int) {
	p.next()
	v, m := p.functionName()
	p.function.StoreVariable(v, p.body(m, line))
	p.function.FixLine(line)
}

func (p *parser) localFunction() {
	p.function.MakeLocalVariable(p.checkName())
	p.function.AdjustLocalVariables(1)
	p.function.LocalVariable(p.body(false, p.lineNumber).info).startPC = pc(len(p.function.f.code))
}

func (p *parser) localStatement() {
	v := 0
	for first := true; first || p.testNext(','); v++ {
		p.function.MakeLocalVariable(p.checkName())
		first = false
	}
	if p.testNext('=') {
		e, n := p.expressionList()
		p.function.AdjustAssignment(v, n, e)
	} else {
		var e exprDesc
		p.function.AdjustAssignment(v, 0, e)
	}
	p.function.AdjustLocalVariables(v)
}

func (p *parser) expressionStatement() {
	if e := p.suffixedExpression(); p.t == '=' || p.t == ',' {
		p.assignment(&assignmentTarget{exprDesc: e}, 1)
	} else {
		p.checkCondition(e.kind == kindCall, "syntax error")
		p.function.Instruction(e).setC(1) // call statement uses no results
	}
}

func (p *parser) returnStatement() {
	if f := p.function; p.blockFollow(true) || p.t == ';' {
		f.ReturnNone()
	} else {
		f.Return(p.expressionList())
	}
	p.testNext(';')
}

func (p *parser) statement() {
	line := p.lineNumber
	p.enterLevel()
	switch p.t {
	case ';':
		p.next()
	case tkIf:
		p.ifStatement(line)
	case tkWhile:
		p.whileStatement(line)
	case tkDo:
		p.next()
		p.block()
		p.checkMatch(tkEnd, tkDo, line)
	case tkFor:
		p.forStatement(line)
	case tkRepeat:
		p.repeatStatement(line)
	case tkFunction:
		p.functionStatement(line)
	case tkLocal:
		p.next()
		if p.testNext(tkFunction) {
			p.localFunction()
		} else {
			p.localStatement()
		}
	case tkDoubleColon:
		p.next()
		p.labelStatement(p.checkName(), line)
	case tkReturn:
		p.next()
		p.returnStatement()
	case tkBreak, tkGoto:
		p.gotoStatement(p.function.Jump())
	default:
		p.expressionStatement()
	}
	p.assert(p.function.f.maxStackSize >= p.function.freeRegisterCount && p.function.freeRegisterCount >= p.function.activeVariableCount)
	p.function.freeRegisterCount = p.function.activeVariableCount
	p.leaveLevel()
}

func (p *parser) mainFunction() {
	p.function.OpenMainFunction()
	p.next()
	p.statementList()
	p.check(tkEOS)
	p.function = p.function.CloseMainFunction()
}

func (l *State) parse(r io.ByteReader, name string) *luaClosure {
	p := &parser{scanner: scanner{r: r, lineNumber: 1, lastLine: 1, lookAheadToken: token{t: tkEOS}, l: l, source: name}}
	f := &function{f: &prototype{source: name, maxStackSize: 2, isVarArg: true}, constantLookup: make(map[value]int), p: p, jumpPC: noJump}
	p.function = f
	p.mainFunction()
	// TODO assertions about parser state
	c := l.newLuaClosure(f.f)
	l.push(c)
	return c
}

func (l *State) checkMode(mode, x string) {
	if mode != "" && !strings.Contains(mode, x[:1]) {
		l.push(fmt.Sprintf("attempt to load a %s chunk (mode is '%s')", x, mode))
		l.throw(SyntaxError)
	}
}

func protectedParser(l *State, r io.Reader, name, chunkMode string) error {
	l.nonYieldableCallCount++
	err := l.protectedCall(func() {
		var closure *luaClosure
		b := bufio.NewReader(r)
		if c, err := b.ReadByte(); err != nil {
			l.checkMode(chunkMode, "text")
			closure = l.parse(b, name)
		} else if c == Signature[0] {
			l.checkMode(chunkMode, "binary")
			b.UnreadByte()
			closure, err = l.undump(b, name) // TODO handle err
		} else {
			l.checkMode(chunkMode, "text")
			b.UnreadByte()
			closure = l.parse(b, name)
		}
		l.assert(closure.upValueCount() == len(closure.prototype.upValues))
		for i := range closure.upValues {
			closure.upValues[i] = l.newUpValue()
		}
	}, l.top, l.errorFunction)
	l.nonYieldableCallCount--
	return err
}
