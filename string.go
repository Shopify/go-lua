package lua

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

func relativePosition(pos, length int) int {
	if pos >= 0 {
		return pos
	} else if -pos > length {
		return 0
	}
	return length + pos + 1
}

func findHelper(l *State, isFind bool) int {
	s, p := CheckString(l, 1), CheckString(l, 2)
	init := relativePosition(OptInteger(l, 3, 1), len(s))
	if init < 1 {
		init = 1
	} else if init > len(s)+1 {
		l.PushNil()
		return 1
	}
	isPlain := l.TypeOf(4) == TypeNone || l.ToBoolean(4)
	if isFind && (isPlain || !strings.ContainsAny(p, "^$*+?.([%-")) {
		if start := strings.Index(s[init-1:], p); start >= 0 {
			l.PushInteger(start + init)
			l.PushInteger(start + init + len(p) - 1)
			return 2
		}
	} else {
		l.assert(false) // TODO implement pattern matching
	}
	l.PushNil()
	return 1
}

func scanFormat(l *State, fs string) string {
	i := 0
	skipDigit := func() {
		if unicode.IsDigit(rune(fs[i])) {
			i++
		}
	}
	flags := "-+ #0"
	for i < len(fs) && strings.ContainsRune(flags, rune(fs[i])) {
		i++
	}
	if i >= len(flags) {
		Errorf(l, "invalid format (repeated flags)")
	}
	skipDigit()
	skipDigit()
	if fs[i] == '.' {
		i++
		skipDigit()
		skipDigit()
	}
	if unicode.IsDigit(rune(fs[i])) {
		Errorf(l, "invalid format (width or precision too long)")
	}
	i++
	return "%" + fs[:i]
}

func formatHelper(l *State, fs string, argCount int) string {
	var b bytes.Buffer
	for i, arg := 0, 1; i < len(fs); i++ {
		if fs[i] != '%' {
			b.WriteByte(fs[i])
		} else if i++; fs[i] == '%' {
			b.WriteByte(fs[i])
		} else {
			if arg++; arg > argCount {
				ArgumentError(l, arg, "no value")
			}
			f := scanFormat(l, fs[i:])
			switch i += len(f) - 2; fs[i] {
			case 'c':
				// Ensure each character is represented by a single byte, while preserving format modifiers.
				c := CheckInteger(l, arg)
				fmt.Fprintf(&b, f, 'x')
				buf := b.Bytes()
				buf[len(buf)-1] = byte(c)
			case 'i': // The fmt package doesn't support %i.
				f = f[:len(f)-1] + "d"
				fallthrough
			case 'd':
				n := CheckNumber(l, arg)
				ni := int(n)
				diff := n - float64(ni)
				ArgumentCheck(l, -1 < diff && diff < 1, arg, "not a number in proper range")
				fmt.Fprintf(&b, f, ni)
			case 'u': // The fmt package doesn't support %u.
				f = f[:len(f)-1] + "d"
				fallthrough
			case 'o', 'x', 'X':
				n := CheckNumber(l, arg)
				ni := uint(n)
				diff := n - float64(ni)
				ArgumentCheck(l, -1 < diff && diff < 1, arg, "not a non-negative number in proper range")
				fmt.Fprintf(&b, f, ni)
			case 'e', 'E', 'f', 'g', 'G':
				fmt.Fprintf(&b, f, CheckNumber(l, arg))
			case 'q':
				s := CheckString(l, arg)
				b.WriteByte('"')
				for i := 0; i < len(s); i++ {
					switch s[i] {
					case '"', '\\', '\n':
						b.WriteByte('\\')
						b.WriteByte(s[i])
					default:
						if 0x20 <= s[i] && s[i] != 0x7f { // ASCII control characters don't correspond to a Unicode range.
							b.WriteByte(s[i])
						} else if i+1 < len(s) && unicode.IsDigit(rune(s[i+1])) {
							fmt.Fprintf(&b, "\\%03d", s[i])
						} else {
							fmt.Fprintf(&b, "\\%d", s[i])
						}
					}
				}
				b.WriteByte('"')
			case 's':
				if s, _ := ToStringMeta(l, arg); !strings.ContainsRune(f, '.') && len(s) >= 100 {
					b.WriteString(s)
				} else {
					fmt.Fprintf(&b, f, s)
				}
			default:
				Errorf(l, fmt.Sprintf("invalid option '%%%c' to 'format'", fs[i]))
			}
		}
	}
	return b.String()
}

var stringLibrary = []RegistryFunction{
	{"byte", func(l *State) int {
		s := CheckString(l, 1)
		start := relativePosition(OptInteger(l, 2, 1), len(s))
		end := relativePosition(OptInteger(l, 3, start), len(s))
		if start < 1 {
			start = 1
		}
		if end > len(s) {
			end = len(s)
		}
		if start > end {
			return 0
		}
		n := end - start + 1
		if start+n <= end {
			Errorf(l, "string slice too long")
		}
		CheckStackWithMessage(l, n, "string slice too long")
		for _, c := range []byte(s[start-1 : end]) {
			l.PushInteger(int(c))
		}
		return n
	}},
	{"char", func(l *State) int {
		var b bytes.Buffer
		for i, n := 1, l.Top(); i <= n; i++ {
			c := CheckInteger(l, i)
			ArgumentCheck(l, int(byte(c)) == c, i, "value out of range")
			b.WriteByte(byte(c))
		}
		l.PushString(b.String())
		return 1
	}},
	// {"dump", ...},
	{"find", func(l *State) int { return findHelper(l, true) }},
	{"format", func(l *State) int {
		l.PushString(formatHelper(l, CheckString(l, 1), l.Top()))
		return 1
	}},
	// {"gmatch", ...},
	// {"gsub", ...},
	{"len", func(l *State) int { l.PushInteger(len(CheckString(l, 1))); return 1 }},
	{"lower", func(l *State) int { l.PushString(strings.ToLower(CheckString(l, 1))); return 1 }},
	// {"match", ...},
	{"rep", func(l *State) int {
		s, n, sep := CheckString(l, 1), CheckInteger(l, 2), OptString(l, 3, "")
		if n <= 0 {
			l.PushString("")
		} else if len(s)+len(sep) < len(s) || len(s)+len(sep) >= maxInt/n {
			Errorf(l, "resulting string too large")
		} else if sep == "" {
			l.PushString(strings.Repeat(s, n))
		} else {
			var b bytes.Buffer
			b.Grow(n*len(s) + (n-1)*len(sep))
			b.WriteString(s)
			for ; n > 1; n-- {
				b.WriteString(sep)
				b.WriteString(s)
			}
			l.PushString(b.String())
		}
		return 1
	}},
	{"reverse", func(l *State) int {
		r := []rune(CheckString(l, 1))
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		l.PushString(string(r))
		return 1
	}},
	{"sub", func(l *State) int {
		s := CheckString(l, 1)
		start, end := relativePosition(CheckInteger(l, 2), len(s)), relativePosition(OptInteger(l, 3, -1), len(s))
		if start < 1 {
			start = 1
		}
		if end > len(s) {
			end = len(s)
		}
		if start <= end {
			l.PushString(s[start-1 : end])
		} else {
			l.PushString("")
		}
		return 1
	}},
	{"upper", func(l *State) int { l.PushString(strings.ToUpper(CheckString(l, 1))); return 1 }},
}

// StringOpen opens the string library. Usually passed to Require.
func StringOpen(l *State) int {
	NewLibrary(l, stringLibrary)
	l.CreateTable(0, 1)
	l.PushString("")
	l.PushValue(-2)
	l.SetMetaTable(-2)
	l.Pop(1)
	l.PushValue(-2)
	l.SetField(-2, "__index")
	l.Pop(1)
	return 1
}
