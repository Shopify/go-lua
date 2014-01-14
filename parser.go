package lua

import "io"

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

type exprDesc struct {
	kind      int
	index     int // register/constant index
	table     int // register or upvalue
	tableType int // whether 'table' is register (kindLocal) or upvalue (kindUpValue)
	info      int
	t, f      int // patch lists for 'exit when true/false'
	value     float64
}

type parser struct {
	scanner
	function function
	r        io.Reader
	// TODO buffer for tokens
	// TODO dynamicData
	source, environment string
	decimalPoint        rune
}

func (e exprDesc) hasJumps() bool {
	return e.t != e.f
}

// func (f *function) enterBlock(isLoop bool) *block {
//   f.block = &block{isLoop: isLoop, activeVariableCount: f.activeVariableCount, firstLabel: f.p.dynamicData.label, firstGoto: f.p.dynamicData.goto, previous: f.block}
//   f.p.state.assert(f.freeRegisters == f.activeVariableCount)
//   return f.block
// }

func (p *parser) syntaxError(message string) {
	p.scanError(message, p.t)
}

func (p *parser) checkCondition(c bool, message string) {
	if !c {
		p.syntaxError(message)
	}
}

func makeExpression(kind, info int) (e exprDesc) {
	e.f, e.t = -1, -1 // TODO noJump
	e.kind, e.info = kind, info
	return
}

func (p *parser) checkName() string {
	p.check(tkName)
	s := p.s
	p.next()
	return s
}

// func (p *parser) codeString(s string) exprDesc {
// 	return makeExpression(kindConstant, p.function.stringConstant(s))
// }

// func (p *parser) checkNameAsExpression() exprDesc {
// 	return p.codeString(p.checkName())
// }

func (p *parser) primaryExpression() (e exprDesc) {
	switch p.t {
	case '(':
		line := p.lineNumber
		p.next()
		e = p.expression()
		p.checkMatch(')', '(', line)
		// p.function.dischargeVariables(v)
	case tkName:
		// e = p.singleVariable()
	default:
		p.syntaxError("unexpected symbol")
	}
	return
}

func (p *parser) suffixedExpression() (e exprDesc) {
	// line := p.lineNumber
	e = p.primaryExpression()
	for {
		switch p.t {
		case '.':
			e = p.fieldSelector(e)
		case '[':
			// p.function.expressionToAnyRegisterUpValue(e)
			// k := p.index()
			// p.function.indexed(e, k)
		case ':':
			p.next()
			// k := p.checkNameAsExpression()
			// p.function.self(e, k)
			// p.functionArguments(e, line)
		case '(', tkString, '{':
			// p.function.expressionToNextRegister(e)
			// p.functionArguments(e, line)
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
		// TODO
	case tkNil:
		e = makeExpression(kindNil, 0)
	case tkTrue:
		e = makeExpression(kindTrue, 0)
	case tkFalse:
		e = makeExpression(kindFalse, 0)
	case tkDots:
		p.checkCondition(p.function.isVarArg, "cannot use '...' outside a vararg function")
		e = makeExpression(kindVarArg, p.function.codeABC(opVarArg, 0, 1, 0))
	case '{':
		// e = p.constructor()
		return
	case tkFunction:
		p.next()
		// TODO
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
	// p.enterLevel()
	if u := unaryOp(p.t); u != oprNoUnary {
		// line := p.lineNumber
		p.next()
		e, _ = p.subExpression(unaryPriority)
		// p.function.prefix(u, line)
	} else {
		e = p.simpleExpression()
	}
	op = binaryOp(p.t)
	for op != oprNoBinary && priority[op].left > limit {
		// line := p.lineNumber
		p.next()
		// p.function.infix(op)
		// e2, next := p.subExpression(priority[op].right)
		// p.function.postfix(op, e, e2, line)
		// op = next
	}
	// p.leaveLevel()
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
	// p.function.expressionToAnyRegisterOrUpValue(e)
	p.next() // skip dot or colon
	// k := p.checkNameAsExpression()
	// e = p.function.indexed(e, k)
	return e
}

func (p *parser) index() exprDesc {
	p.next() // skip '['
	e := p.expression()
	// p.function.expressionToValue(e)
	p.checkNext("]")
	return e
}

// func (p *parser) testThenBlock(escapes int) int {
//   p.next()
//   v := p.expression()
//   p.checkNext(tkThen)
//   if p.t == tkGoto || p.t == tkBreak {
//     p.function.goIfFalse(v)
//     b
//   }
// }

// func (p *parser) ifStatement(line int) {
// 	escapes := p.testThenBlock(noJump)
// 	for p.t == tkElseif {
// 		escapes = p.testThenBlock(escapes)
// 	}
// 	if p.testNext(tkElse) {
// 		p.block()
// 	}
// 	p.checkMatch(tkEnd, tkIf, line)
// 	p.function.patchToHere(escapes)
// }

func (p *parser) statement() {
	line := p.lineNumber
	// p.enterLevel()
	switch p.t {
	case ';':
		p.next()
	case tkIf:
		// p.ifStatement(line)
	case tkWhile:
		// p.whileStatement(line)
	case tkDo:
		p.next()
		// p.block()
		p.checkMatch(tkEnd, tkDo, line)
	case tkFor:
		// p.forStatement(line)
	case tkRepeat:
		// p.repeatStatement(line)
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
		// p.labelStatement(p.checkName(), line)
	case tkReturn:
		p.next()
		// p.returnStatement()
	case tkBreak, tkGoto:
		// p.gotoStatement(p.f.jump())
	default:
		// p.expressionStatement()
	}
	// p.l.assert(...)
	// p.f.freeRegisters = p.f.activeVariableCount
	// p.leaveLevel()
}
