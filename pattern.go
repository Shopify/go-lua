package lua

import (
	"bytes"
	"strings"
	"unicode"
)

const luaMaxCaptures = 32

const (
	capUnfinished = -1
	capPosition   = -2
)

type matchState struct {
	matchDepth int
	src        *string
	p          *string
	l          *State
	level      int
	capture    [luaMaxCaptures]struct {
		init int
		len  int
	}
}

const maxCCalls = 200
const lEsc = '%'
const specials = "^$*+?.([%-"

func checkCapture(ms *matchState, l int) int {
	l = l - '1'
	if l < 0 || l >= ms.level || ms.capture[l].len == capUnfinished {
		Errorf(ms.l, "invalid capture index %%%d", l+1)
	}
	return l
}

func captureToClose(ms *matchState) int {
	level := ms.level
	level--
	for ; level >= 0; level-- {
		if ms.capture[level].len == capUnfinished {
			return level
		}
	}
	Errorf(ms.l, "invalid pattern capture")
	return 0
}

func classend(ms *matchState, ppos int) int {
	switch (*ms.p)[ppos] {
	case lEsc:
		ppos++
		if ppos >= len(*ms.p) {
			Errorf(ms.l, "malformed pattern (ends with '%%')")
		}
		return ppos + 1
	case '[':
		ppos++
		if (*ms.p)[ppos] == '^' {
			ppos++
		}
		for { // look for a ']'
			if ppos >= len(*ms.p) {
				Errorf(ms.l, "malformed pattern (missing ']')")
			}
			ppos++
			if ppos < len(*ms.p) && (*ms.p)[ppos] == lEsc {
				ppos = ppos + 2 // skip escapes (e.g. `%]')
			}
			if ppos < len(*ms.p) && (*ms.p)[ppos] == ']' {
				break
			}
		}
		return ppos + 1
	default:
		return ppos + 1
	}
}

func matchClass(c byte, cl byte) bool {
	var res bool
	var rc, rcl rune = rune(c), rune(cl)
	switch unicode.ToLower(rcl) {
	case 'a':
		res = unicode.IsLetter(rc)
	case 'c':
		res = unicode.IsControl(rc)
	case 'd':
		res = unicode.IsDigit(rc)
	case 'g':
		res = unicode.IsGraphic(rc) && !unicode.IsSpace(rc)
	case 'l':
		res = unicode.IsLower(rc)
	case 'p':
		res = unicode.In(rc, unicode.Mark, unicode.Punct, unicode.Symbol)
	case 's':
		res = unicode.IsSpace(rc)
	case 'u':
		res = unicode.IsUpper(rc)
	case 'w':
		res = unicode.In(rc, unicode.Letter, unicode.Number)
	case 'x':
		res = unicode.In(rc, unicode.Hex_Digit)
	case 'z':
		res = (c == 0)
	default:
		return cl == c
	}
	if unicode.IsLower(rcl) {
		return res
	} else {
		return !res
	}
}

func matchbracketclass(c byte, p string, ppos int, ecpos int) bool {
	sig := true

	if p[ppos+1] == '^' {
		sig = false
		ppos++ // skip the `^'
	}

	for {
		ppos++
		if ppos >= ecpos {
			break
		}

		if p[ppos] == lEsc {
			ppos++
			if matchClass(c, p[ppos]) {
				return sig
			}
		} else if p[ppos+1] == '-' && ppos+2 < ecpos {
			ppos = ppos + 2
			if p[ppos-2] <= c && c <= p[ppos] {
				return sig
			}
		} else if p[ppos] == c {
			return sig
		}
	}

	return !sig
}

func singlematch(ms *matchState, spos int, ppos int, eppos int) bool {
	if spos >= len(*ms.src) {
		return false
	} else {
		var c byte = (*ms.src)[spos]
		switch (*ms.p)[ppos] {
		case '.':
			return true // matches any char
		case lEsc:
			return matchClass(c, (*ms.p)[ppos+1])
		case '[':
			return matchbracketclass(c, *ms.p, ppos, eppos-1)
		default:
			return (*ms.p)[ppos] == c
		}
	}
}

func matchbalance(ms *matchState, spos int, ppos int) (int, bool) {
	if ppos >= len(*ms.p)-1 {
		Errorf(ms.l, "malformed pattern (missing arguments to '%%b')")
	}

	if spos >= len(*ms.src) || (*ms.src)[spos] != (*ms.p)[ppos] {
		return 0, false
	} else {
		b := (*ms.p)[ppos]
		e := (*ms.p)[ppos+1]
		cont := 1
		for {
			spos++
			if spos >= len(*ms.src) {
				break
			}
			if (*ms.src)[spos] == e {
				cont--
				if cont == 0 {
					return spos + 1, true
				}
			} else if (*ms.src)[spos] == b {
				cont++
			}
		}
	}

	return 0, false
}

func maxExpand(ms *matchState, spos int, ppos int, eppos int) (int, bool) {
	i := 0 // counts maximum expand for item
	for {
		if singlematch(ms, spos+i, ppos, eppos) {
			i++
		} else {
			break
		}
	}
	// keeps trying to match with the maximum repetitions
	for {
		if i < 0 {
			break
		}
		res, ok := match(ms, spos+i, eppos+1)
		if ok {
			return res, ok
		}
		i--
	}
	return 0, false
}

func minExpand(ms *matchState, spos int, ppos int, eppos int) (int, bool) {
	for {
		res, ok := match(ms, spos, eppos+1)
		if ok {
			return res, true
		} else if singlematch(ms, spos, ppos, eppos) {
			spos++
		} else {
			return 0, false
		}
	}
}

func startCapture(ms *matchState, spos int, ppos int, what int) (int, bool) {
	level := ms.level
	if level >= luaMaxCaptures {
		Errorf(ms.l, "too many captures")
	}
	ms.capture[level].init = spos
	ms.capture[level].len = what
	ms.level = level + 1
	res, ok := match(ms, spos, ppos)
	if !ok { // match failed?
		ms.level-- // undo capture
	}
	return res, ok
}

func endCapture(ms *matchState, spos int, ppos int) (int, bool) {
	l := captureToClose(ms)
	ms.capture[l].len = spos - ms.capture[l].init // close capture
	res, ok := match(ms, spos, ppos)
	if !ok { // match failed?
		ms.capture[l].len = capUnfinished // undo capture
	}
	return res, ok
}

func matchCapture(ms *matchState, spos int, l int) (int, bool) {
	l = checkCapture(ms, l)
	ln := ms.capture[l].len

	// memcmp(ms->capture[l].init, s, len)
	capBytes := (*ms.src)[ms.capture[l].init : ms.capture[l].init+ln]
	sposln := len(*ms.src) - spos
	if ln < sposln {
		sposln = ln
	}
	sposBytes := (*ms.src)[spos : spos+sposln]

	if len(*ms.src)-spos >= ln && strings.Compare(capBytes, sposBytes) == 0 {
		return spos + ln, true
	} else {
		return 0, false
	}
}

// This function makes liberal use of goto in order to keep control over the
// stack size, similar to the original C version of the function.  However,
// this implementation has an additional goto label that was not in the
// original code.  Go cannot jump from one block to another, so the dflt label
// that used to come right after the default case of the main switch could
// not be jumped into.
//
// Instead, we drag the default case outside of the switch, and skip over it
// to the "end" of the function in cases where we shouldn't execute it.
func match(ms *matchState, spos int, ppos int) (int, bool) {
	if ms.matchDepth == 0 {
		Errorf(ms.l, "pattern too complex")
	}
	ms.matchDepth--
	ok := true

init: // using goto's to optimize tail recursion
	if ppos != len(*ms.p) { // end of pattern?
		switch (*ms.p)[ppos] {
		case '(': // start capture
			if (*ms.p)[ppos+1] == ')' {
				spos, ok = startCapture(ms, spos, ppos+2, capPosition)
			} else {
				spos, ok = startCapture(ms, spos, ppos+1, capUnfinished)
			}
		case ')': // end capture
			spos, ok = endCapture(ms, spos, ppos+1)
		case '$':
			if ppos+1 != len(*ms.p) { // is the `$' the last char in pattern?
				goto dflt
			} else {
				if spos != len(*ms.src) {
					spos, ok = 0, false
				}
			}
		case lEsc:
			var pnext byte
			if ppos+1 < len(*ms.p) {
				pnext = (*ms.p)[ppos+1]
			}
			switch {
			case pnext == 'b': // balanced string?
				spos, ok = matchbalance(ms, spos, ppos+2)
				if ok {
					ppos = ppos + 4
					goto init // return match(ms, s, p + 4)
				} // else fail
			case pnext == 'f': // frontier?
				ppos = ppos + 2
				if ppos >= len(*ms.p) || (*ms.p)[ppos] != '[' {
					Errorf(ms.l, "missing '[' after '%%f' in pattern")
				}
				eppos := classend(ms, ppos) // points to what is next
				var previous byte = 0
				if spos != 0 {
					previous = (*ms.src)[spos-1]
				}
				if !matchbracketclass(previous, *ms.p, ppos, eppos-1) {
					var sc byte
					if spos < len(*ms.src) {
						sc = (*ms.src)[spos]
					}
					if matchbracketclass(sc, *ms.p, ppos, eppos-1) {
						ppos = eppos
						goto init
					}
				}
				ok = false // match failed
			case pnext >= '0' && pnext <= '9': /* capture results (%0-%9)? */
				spos, ok = matchCapture(ms, spos, int((*ms.p)[ppos+1]))
				if ok {
					ppos = ppos + 2
					goto init // return match(ms, s, p + 2)
				}
			default:
				goto dflt
			}
		default:
			goto dflt // Old dflt label was here.
		}
		goto end // We shouldn't execute the default case.
	dflt: // pattern class plus optional suffix
		eppos := classend(ms, ppos) // points to optional suffix
		// does not match at least once?
		if !singlematch(ms, spos, ppos, eppos) {
			var ep byte = 0
			if eppos != len(*ms.p) {
				ep = (*ms.p)[eppos]
			}
			if ep == '*' || ep == '?' || ep == '-' { // accept empty?
				ppos = eppos + 1
				goto init // return match(ms, spos, eppos + 1);
			} else { // '+' or no suffix
				ok = false // fail
			}
		} else { // matched once
			var ep byte = 0
			if eppos != len(*ms.p) {
				ep = (*ms.p)[eppos]
			}
			switch ep {
			case '?': // optional
				res, resOk := match(ms, spos+1, eppos+1)
				if resOk {
					spos = res
				} else {
					ppos = eppos + 1
					goto init
				}
			case '+': // 1 or more repetitions
				spos++ // 1 match already done
				fallthrough
			case '*': // 0 or more repetitions
				spos, ok = maxExpand(ms, spos, ppos, eppos)
			case '-': // 0 or more repetitions (minimum)
				spos, ok = minExpand(ms, spos, ppos, eppos)
			default: // no suffix
				spos++
				ppos = eppos
				goto init
			}
		}
	}
end:
	ms.matchDepth++
	return spos, ok
}

func pushOnecapture(ms *matchState, i int, spos int, epos int) {
	if i >= ms.level {
		if i == 0 { // ms->level == 0, too
			ms.l.PushString((*ms.src)[spos:epos]) // add whole match
		} else {
			Errorf(ms.l, "invalid capture index")
		}
	} else {
		l := ms.capture[i].len
		if l == capUnfinished {
			Errorf(ms.l, "unfinished capture")
		}
		ipos := ms.capture[i].init
		if l == capPosition {
			ms.l.PushInteger(ipos + 1)
		} else {
			ms.l.PushString((*ms.src)[ipos : ipos+l])
		}
	}
}

func pushCaptures(ms *matchState, spos int, epos int, snil bool) int {
	nlevels := 1
	if !(ms.level == 0 && !snil) {
		nlevels = ms.level
	}
	CheckStackWithMessage(ms.l, nlevels, "too many captures")
	for i := 0; i < nlevels; i++ {
		pushOnecapture(ms, i, spos, epos)
	}
	return nlevels
}

func nospecials(p string) bool {
	if strings.IndexAny(p, specials) != -1 {
		return false
	}
	return true
}

func strFindAux(l *State, find bool) int {
	s := CheckString(l, 1)
	p := CheckString(l, 2)

	init := relativePosition(OptInteger(l, 3, 1), len(s))
	if init < 1 {
		init = 1
	} else if init > len(s)+1 { // start after string's end?
		l.PushNil() // cannot find anything
		return 1
	}
	// explicit request or no special characters?
	if find && l.ToBoolean(4) || nospecials(p) {
		// do a plain search
		s2 := strings.Index(s[init-1:], p)
		if s2 != -1 {
			l.PushInteger(s2 + init)
			l.PushInteger(s2 + init + len(p) - 1)
			return 2
		}
	} else {
		s1 := init - 1
		anchor := p[0] == '^'
		if anchor {
			p = p[1:] // skip anchor character
		}

		ms := matchState{
			l:          l,
			matchDepth: maxCCalls,
			src:        &s,
			p:          &p,
		}

		for {
			ms.level = 0
			res, ok := match(&ms, s1, 0)
			if ok {
				if find {
					l.PushInteger(s1 + 1)
					l.PushInteger(res)
					return pushCaptures(&ms, 0, 0, true) + 2
				} else {
					return pushCaptures(&ms, s1, res, false)
				}
			}

			if !(s1 < len(*ms.src) && !anchor) {
				break
			}
			s1++
		}
	}

	l.PushNil()
	return 1
}

func strFind(l *State) int {
	return strFindAux(l, true)
}

func strMatch(l *State) int {
	return strFindAux(l, false)
}

func gmatchAux(l *State) int {
	s, _ := l.ToString(UpValueIndex(1))
	p, _ := l.ToString(UpValueIndex(2))

	ms := matchState{
		l:          l,
		matchDepth: maxCCalls,
		src:        &s,
		p:          &p,
	}

	srcpos, _ := l.ToInteger(UpValueIndex(3))
	for ; srcpos <= len(*ms.src); srcpos++ {
		ms.level = 0
		epos, ok := match(&ms, srcpos, 0)
		if ok {
			newstart := epos
			if epos == srcpos {
				newstart++
			}
			l.PushInteger(newstart)
			l.Replace(UpValueIndex(3))
			return pushCaptures(&ms, srcpos, epos, false)
		}
	}
	return 0
}

func gmatch(l *State) int {
	CheckString(l, 1)
	CheckString(l, 2)
	l.SetTop(2)
	l.PushInteger(0)
	l.PushGoClosure(gmatchAux, 3)
	return 1
}

func addS(ms *matchState, b *bytes.Buffer, spos int, epos int) {
	news, _ := ms.l.ToString(3)
	for i := 0; i < len(news); i++ {
		if news[i] != lEsc {
			b.WriteByte(news[i])
		} else {
			i++ // skip ESC
			if !unicode.IsDigit(rune(news[i])) {
				if news[i] != lEsc {
					Errorf(ms.l, "invalid use of '%%' in replacement string")
				}
				b.WriteByte(news[i])
			} else if news[i] == '0' {
				b.WriteString((*ms.src)[spos:epos])
			} else {
				pushOnecapture(ms, int(news[i]-'1'), spos, epos)
				bs, _ := ms.l.ToString(-1) // add capture to accumulated result
				b.WriteString(bs)
				ms.l.Pop(1)
			}
		}
	}
}

func addValue(ms *matchState, b *bytes.Buffer, spos int, epos int, tr Type) {
	switch tr {
	case TypeFunction:
		ms.l.PushValue(3)
		n := pushCaptures(ms, spos, epos, false)
		ms.l.Call(n, 1)
	case TypeTable:
		pushOnecapture(ms, 0, spos, epos)
		ms.l.Table(3)
	default: // TypeNumber or TypeString
		addS(ms, b, spos, epos)
		return
	}

	if !ms.l.ToBoolean(-1) { // nil or false?
		ms.l.Pop(1)
		ms.l.PushString((*ms.src)[spos:epos]) // keep original text
	} else if !ms.l.IsString(-1) {
		Errorf(ms.l, "invalid replacement value (a %s)", TypeNameOf(ms.l, -1))
	}

	bs, _ := ms.l.ToString(-1) // add result to accumulator
	b.WriteString(bs)
	ms.l.Pop(1)
}

func strGsub(l *State) int {
	src := CheckString(l, 1)
	p := CheckString(l, 2)
	tr := l.TypeOf(3)
	maxS := OptInteger(l, 4, len(src)+1)

	anchor := len(p) > 0 && p[0] == '^'
	n := 0

	ArgumentCheck(l, tr == TypeNumber || tr == TypeString || tr == TypeFunction || tr == TypeTable, 3, "string/function/table expected")
	if anchor {
		p = p[1:] // skip anchor character
	}

	ms := matchState{
		l:          l,
		matchDepth: maxCCalls,
		src:        &src,
		p:          &p,
	}
	srcpos := 0
	b := new(bytes.Buffer)

	for {
		if n >= maxS {
			break
		}

		ms.level = 0
		epos, ok := match(&ms, srcpos, 0)
		if ok {
			n++
			addValue(&ms, b, srcpos, epos, tr)
		}
		if ok && epos > srcpos { // non empty match?
			srcpos = epos // skip it
		} else if srcpos < len(src) {
			b.WriteByte(src[srcpos])
			srcpos++
		} else {
			break
		}
		if anchor {
			break
		}
	}

	b.WriteString(src[srcpos:])
	l.PushString(b.String())
	l.PushInteger(n) // number of substitutions

	return 2
}
