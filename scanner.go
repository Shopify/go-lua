package lua

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
)

const firstReserved = 257
const endOfStream = -1
const maxInt = int(^uint(0) >> 1)

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

var tokens []string = []string{
	"and", "break", "do", "else", "elseif",
	"end", "false", "for", "function", "goto", "if",
	"in", "local", "nil", "not", "or", "repeat",
	"return", "then", "true", "until", "while",
	"..", "...", "==", ">=", "<=", "~=", "::", "<eof>",
	"<number>", "<name>", "<string>",
}

type token struct {
	t rune
	n float64
	s string
}

type scanner struct {
	l                    *State
	buffer               bytes.Buffer
	r                    io.ByteReader
	current              rune
	lineNumber, lastLine int
	source               string
	lookAheadToken       token
	token
}

func (s *scanner) assert(cond bool)           { s.l.assert(cond) }
func (s *scanner) syntaxError(message string) { s.scanError(message, s.t) }
func (s *scanner) errorExpected(t rune)       { s.syntaxError(s.tokenToString(t) + " expected") }
func (s *scanner) numberError()               { s.scanError("malformed number", tkNumber) }
func isNewLine(c rune) bool                   { return c == '\n' || c == '\r' }
func isDecimal(c rune) bool                   { return '0' <= c && c <= '9' }

func (s *scanner) tokenToString(t rune) string {
	switch {
	case t == tkName || t == tkString:
		return s.s
	case t == tkNumber:
		return fmt.Sprintf("%f", s.n)
	case t < firstReserved:
		return string(t) // TODO check for printable rune
	case t < tkEOS:
		return fmt.Sprintf("'%s'", tokens[t-firstReserved])
	}
	return tokens[t-firstReserved]
}

func (s *scanner) scanError(message string, token rune) {
	buff := chunkID(s.source)
	if token != 0 {
		message = fmt.Sprintf("%s:%d: %s near %s", buff, s.lineNumber, message, s.tokenToString(token))
	} else {
		message = fmt.Sprintf("%s:%d: %s", buff, s.lineNumber, message)
	}
	s.l.push(message)
	s.l.throw(SyntaxError)
}

func (s *scanner) incrementLineNumber() {
	old := s.current
	s.assert(isNewLine(old))
	if s.advance(); isNewLine(s.current) && s.current != old {
		s.advance()
	}
	if s.lineNumber++; s.lineNumber >= maxInt {
		s.syntaxError("chunk has too many lines")
	}
}

func (s *scanner) advance() {
	if c, err := s.r.ReadByte(); err != nil {
		s.current = endOfStream
	} else {
		s.current = rune(c)
	}
}

func (s *scanner) saveAndAdvance() {
	s.save(s.current)
	s.advance()
}

func (s *scanner) advanceAndSave(c rune) {
	s.advance()
	s.save(c)
}

func (s *scanner) save(c rune) {
	if err := s.buffer.WriteByte(byte(c)); err != nil {
		s.scanError("lexical element too long", 0)
	}
}

func (s *scanner) checkNext(str string) bool {
	if s.current == 0 || !strings.ContainsRune(str, s.current) {
		return false
	}
	s.saveAndAdvance()
	return true
}

func (s *scanner) skipSeparator() int { // TODO is this the right name?
	i, c := 0, s.current
	s.assert(c == '[' || c == ']')
	for s.saveAndAdvance(); s.current == '='; i++ {
		s.saveAndAdvance()
	}
	if s.current == c {
		return i
	}
	return -i - 1
}

func (s *scanner) readMultiLine(comment bool, sep int) (str string) {
	if s.saveAndAdvance(); isNewLine(s.current) {
		s.incrementLineNumber()
	}
	for {
		switch s.current {
		case endOfStream:
			if comment {
				s.scanError("unfinished long comment", tkEOS)
			} else {
				s.scanError("unfinished long string", tkEOS)
			}
		case ']':
			if s.skipSeparator() == sep {
				s.saveAndAdvance()
				if !comment {
					str = s.buffer.String()
					str = str[2+sep : len(str)-(2+sep)]
				}
				s.buffer.Reset()
				return
			}
		case '\r':
			s.current = '\n'
			fallthrough
		case '\n':
			s.save(s.current)
			s.incrementLineNumber()
		default:
			if !comment {
				s.save(s.current)
			}
			s.advance()
		}
	}
}

func (s *scanner) readDigits() (c rune) {
	for c = s.current; isDecimal(c); c = s.current {
		s.saveAndAdvance()
	}
	return
}

func isHexadecimal(c rune) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func (s *scanner) readHexNumber(x float64) (n float64, c rune, i int) {
	if c, n = s.current, x; !isHexadecimal(c) {
		return
	}
	for {
		switch {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return
		}
		s.advance()
		c, n, i = s.current, n*16.0+float64(c), i+1
	}
}

func (s *scanner) readNumber() token {
	const bits64, base10 = 64, 10
	c := s.current
	s.assert(isDecimal(c))
	s.saveAndAdvance()
	if c == '0' && s.checkNext("Xx") { // hexadecimal
		prefix := s.buffer.String()
		s.assert(prefix == "0x" || prefix == "0X")
		s.buffer.Reset()
		var exponent int
		fraction, c, i := s.readHexNumber(0)
		if c == '.' {
			s.advance()
			fraction, c, exponent = s.readHexNumber(fraction)
		}
		if i == 0 && exponent == 0 {
			s.numberError()
		}
		exponent *= -4
		if c == 'p' || c == 'P' {
			s.advance()
			var negativeExponent bool
			if c = s.current; c == '+' || c == '-' {
				negativeExponent = c == '-'
				s.advance()
			}
			if !isDecimal(s.current) {
				s.numberError()
			}
			_ = s.readDigits()
			if e, err := strconv.ParseInt(s.buffer.String(), base10, bits64); err != nil {
				s.numberError()
			} else if negativeExponent {
				exponent += int(-e)
			} else {
				exponent += int(e)
			}
			s.buffer.Reset()
		}
		return token{t: tkNumber, n: math.Ldexp(fraction, exponent)}
	}
	c = s.readDigits()
	if c == '.' {
		s.saveAndAdvance()
		c = s.readDigits()
	}
	if c == 'e' || c == 'E' {
		s.saveAndAdvance()
		if c = s.current; c == '+' || c == '-' {
			s.saveAndAdvance()
		}
		_ = s.readDigits()
	}
	str := s.buffer.String()
	if strings.HasPrefix(str, "0") {
		if str = strings.TrimLeft(str, "0"); str == "" || !isDecimal(rune(str[0])) {
			str = "0" + str
		}
	}
	f, err := strconv.ParseFloat(str, bits64)
	if err != nil {
		s.numberError()
	}
	s.buffer.Reset()
	return token{t: tkNumber, n: f}
}

var escapes map[rune]rune = map[rune]rune{
	'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', '\\': '\\', '"': '"', '\'': '\'',
}

func (s *scanner) escapeError(c []rune, message string) {
	s.buffer.Reset()
	s.save('\\')
	for _, r := range c {
		if r == endOfStream {
			break
		}
		s.save(r)
	}
	s.scanError(message, tkString)
}

func (s *scanner) readHexEscape() (r rune) {
	s.advance()
	for i, c, b := 1, s.current, [3]rune{'x'}; i < len(b); i, c, r = i+1, s.current, r<<4+c {
		switch b[i] = c; {
		case '0' <= c && c <= '9':
			c = c - '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			s.escapeError(b[:i+1], "hexadecimal digit expected")
		}
		s.advance()
	}
	return
}

func (s *scanner) readDecimalEscape() (r rune) {
	b := [3]rune{}
	for c, i := s.current, 0; i < len(b) && isDecimal(c); i, c = i+1, s.current {
		b[i], r = c, 10*r+c-'0'
		s.advance()
	}
	if r > math.MaxUint8 {
		s.escapeError(b[:], "decimal escape too large")
	}
	return
}

func (s *scanner) readString() token {
	delimiter := s.current
	for s.saveAndAdvance(); s.current != delimiter; {
		switch s.current {
		case endOfStream:
			s.scanError("unfinished string", tkEOS)
		case '\n', '\r':
			s.scanError("unfinished string", tkString)
		case '\\':
			s.advance()
			c := s.current
			switch esc, ok := escapes[c]; {
			case ok:
				s.advanceAndSave(esc)
			case isNewLine(c):
				s.incrementLineNumber()
				s.save('\n')
			case c == endOfStream: // do nothing
			case c == 'x':
				s.save(s.readHexEscape())
			case c == 'z':
				for s.advance(); unicode.IsSpace(s.current); {
					if isNewLine(s.current) {
						s.incrementLineNumber()
					} else {
						s.advance()
					}
				}
			default:
				if !isDecimal(c) {
					s.escapeError([]rune{c}, "invalid escape sequence")
				}
				s.save(s.readDecimalEscape())
			}
		default:
			s.saveAndAdvance()
		}
	}
	s.saveAndAdvance()
	str := s.buffer.String()
	s.buffer.Reset()
	return token{t: tkString, s: str[1 : len(str)-1]}
}

func isReserved(s string) bool {
	for _, reserved := range tokens[:reservedCount] {
		if s == reserved {
			return true
		}
	}
	return false
}

func (s *scanner) reservedOrName() token {
	str := s.buffer.String()
	s.buffer.Reset()
	for i, reserved := range tokens[:reservedCount] {
		if str == reserved {
			return token{t: rune(i + firstReserved), s: reserved}
		}
	}
	return token{t: tkName, s: str}
}

func (s *scanner) scan() token {
	const comment, str = true, false
	for {
		switch c := s.current; c {
		case '\n', '\r':
			s.incrementLineNumber()
		case ' ', '\f', '\t', '\v':
			s.advance()
		case '-':
			if s.advance(); s.current != '-' {
				return token{t: '-'}
			}
			if s.advance(); s.current == '[' {
				if sep := s.skipSeparator(); sep >= 0 {
					_ = s.readMultiLine(comment, sep)
					break
				}
				s.buffer.Reset()
			}
			for !isNewLine(s.current) && s.current != endOfStream {
				s.advance()
			}
		case '[':
			if sep := s.skipSeparator(); sep >= 0 {
				return token{t: tkString, s: s.readMultiLine(str, sep)}
			} else if s.buffer.Reset(); sep == -1 {
				return token{t: '['}
			}
			s.scanError("invalid long string delimiter", tkString)
		case '=':
			if s.advance(); s.current != '=' {
				return token{t: '='}
			}
			s.advance()
			return token{t: tkEq}
		case '<':
			if s.advance(); s.current != '=' {
				return token{t: '<'}
			}
			s.advance()
			return token{t: tkLE}
		case '>':
			if s.advance(); s.current != '=' {
				return token{t: '>'}
			}
			s.advance()
			return token{t: tkGE}
		case '~':
			if s.advance(); s.current != '=' {
				return token{t: '~'}
			}
			s.advance()
			return token{t: tkNE}
		case ':':
			if s.advance(); s.current != ':' {
				return token{t: ':'}
			}
			s.advance()
			return token{t: tkDoubleColon}
		case '"', '\'':
			return s.readString()
		case endOfStream:
			return token{t: tkEOS}
		case '.':
			if s.saveAndAdvance(); s.checkNext(".") {
				if s.checkNext(".") {
					s.buffer.Reset()
					return token{t: tkDots}
				}
				s.buffer.Reset()
				return token{t: tkConcat}
			} else if !unicode.IsDigit(s.current) {
				s.buffer.Reset()
				return token{t: '.'}
			} else {
				return s.readNumber()
			}
		case 0:
			s.advance()
		default:
			if unicode.IsDigit(c) {
				return s.readNumber()
			} else if c == '_' || unicode.IsLetter(c) {
				for ; c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c); c = s.current {
					s.saveAndAdvance()
				}
				return s.reservedOrName()
			}
			s.advance()
			return token{t: c}
		}
	}
	panic("unreachable")
}

func (s *scanner) next() {
	s.lastLine = s.lineNumber
	if s.lookAheadToken.t != tkEOS {
		s.token = s.lookAheadToken
		s.lookAheadToken.t = tkEOS
	} else {
		s.token = s.scan()
	}
}

func (s *scanner) lookAhead() rune {
	s.l.assert(s.lookAheadToken.t == tkEOS)
	s.lookAheadToken = s.scan()
	return s.lookAheadToken.t
}

func (s *scanner) testNext(t rune) (r bool) {
	if r = s.t == t; r {
		s.next()
	}
	return
}

func (s *scanner) check(t rune) {
	if s.t != t {
		s.errorExpected(t)
	}
}

func (s *scanner) checkMatch(what, who rune, where int) {
	if !s.testNext(what) {
		if where == s.lineNumber {
			s.errorExpected(what)
		} else {
			s.syntaxError(fmt.Sprintf("%s expected (to close %s at line %d)", s.tokenToString(what), s.tokenToString(who), where))
		}
	}
}
