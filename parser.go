package lua

import (
	"fmt"
	"io"
)

type parser struct {
	scanner
	function                   *function
	r                          io.Reader
	activeVariables            []int
	pendingGotos, activeLabels []label
	source, environment        string
	decimalPoint               rune
	// TODO buffer for tokens
}

func (p *parser) syntaxError(message string) {
	p.scanError(message, p.t)
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

func (p *parser) checkNameAsExpression() exprDesc {
	return p.function.EncodeString(p.checkName())
}

func (p *parser) enterLevel() {
	p.l.nestedGoCallCount++
	p.checkLimit(p.l.nestedGoCallCount, maxCallCount, "Go levels")
}

func (p *parser) leaveLevel() {
	p.l.nestedGoCallCount--
}

func (p *parser) expressionList() (e exprDesc, n int) {
	for n, e = 1, p.expression(); p.testNext(','); n, e = n+1, p.expression() {
		_ = p.function.ExpressionToNextRegister(e)
	}
	return
}

func (p *parser) functionArguments(f exprDesc, line int) exprDesc {
	var args exprDesc
	switch p.t {
	case '(':
		p.next()
		if p.t == ')' {
			args.kind = kindVoid
		} else {
			var resultCount int
			args, resultCount = p.expressionList()
			p.function.SetReturns(args, resultCount)
		}
		p.checkMatch(')', '(', line)
	case '{':
		// args = p.constructor()
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
		// e = p.singleVariable()
	default:
		p.syntaxError("unexpected symbol")
	}
	return
}

func (p *parser) suffixedExpression() (e exprDesc) {
	line := p.lineNumber
	e = p.primaryExpression()
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
			return
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
		p.checkCondition(p.function.isVarArg, "cannot use '...' outside a vararg function")
		e = makeExpression(kindVarArg, p.function.EncodeABC(opVarArg, 0, 1, 0))
	case '{':
		// e = p.constructor()
		return
	case tkFunction:
		p.next()
		// TODO e = p.body(false, p.lineNumber)
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
		p.function.Postfix(op, e, e2, line)
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
	p.checkNext("]")
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
		p.checkNext("=")
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

func (p *parser) testThenBlock(escapes int) int {
	var jumpFalse int
	p.next()
	e := p.expression()
	p.checkNext(string(tkThen))
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
	p.checkNext(string(tkDo))
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
	p.checkNext(string(tkDoubleColon))
	l := p.function.MakeLabel(label, line)
	p.skipEmptyStatements()
	if p.blockFollow(false) {
		p.activeLabels[l].activeVariableCount = p.function.block.activeVariableCount
	}
	p.function.FindGotos(l)
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
		// p.forStatement(line)
	case tkRepeat:
		p.repeatStatement(line)
	case tkFunction:
		// p.functionStatement(line)
	case tkLocal:
		p.next()
		if p.testNext(tkFunction) {
			// p.localFunction()
		} else {
			// p.localStatement()
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
