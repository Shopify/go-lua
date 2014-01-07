package lua

const firstReserved = 257

const (
	tkAnd = iota + firstReserved
	tkBreak
	tkDo
	tkElse
	tkElseif
	tkEnd
	tkFalse
	tkFor
	tkFunction
	tkGoto
	tkIf
	tkIn
	tkLocal
	tkNil
	tkNot
	tkOr
	tkRepeat
	tkReturn
	tkThen
	tkTrue
	tkUntil
	tkWhile
	tkConcat
	tkDots
	tkEq
	tkGE
	tkLE
	tkNE
	tkDoubleColon
	tkEOS
	tkNumber
	tkName
	tkString
	reservedCount = tkWhile - firstReserved + 1
)

type token struct {
	t int
	n float64
	s string
}

type scanner struct {
	l                    *state
	current              rune
	lineNumber, lastLine int
	token
	lookAheadToken token
}

func (l *scanner) scan() token {
	var t token
	return t
}

func (l *scanner) next() {
	l.lastLine = l.lineNumber
	if l.lookAheadToken.t != tkEOS {
		l.token = l.lookAheadToken
		l.lookAheadToken.t = tkEOS
	} else {
		l.token = l.scan()
	}
}

func (l *scanner) lookAhead() int {
	l.l.assert(l.lookAheadToken.t == tkEOS)
	l.lookAheadToken = l.scan()
	return l.lookAheadToken.t
}

func (l *scanner) testNext(t int) bool {
	if l.t == t {
		l.next()
		return true
	}
	return false
}

func (l *scanner) errorExpected(t int) {
	// TODO
	panic("unreachable")
}

func (l *scanner) check(t int) {
	if l.t != t {
		l.errorExpected(t)
	}
}

func (l *scanner) checkMatch(what, who, where int) {
	if !l.testNext(what) {
		if where == l.lineNumber {
			l.errorExpected(what)
		} else {
			// TODO
			panic("unreachable")
		}
	}
}
