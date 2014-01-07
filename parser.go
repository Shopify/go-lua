package lua

import "io"

type exprDesc struct {
	kind int
	// TODO ...
}

type function struct {
	f        *prototype
	h        *table
	previous *function
	p        *parser
	// TODO ...
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

func (p *parser) blockFollow(withUntil bool) bool {
	switch p.t {
	case tkElse, tkElseif, tkEnd, tkEOS:
		return true
	case tkUntil:
		return withUntil
	}
	return false
}

func (p *parser) statementList(v *exprDesc) {
	for !p.blockFollow(true) {
		if p.t == tkReturn {
			p.statement()
			return
		}
		p.statement()
	}
}

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
