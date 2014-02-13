package lua

import (
	"fmt"
	"strings"
	"testing"
)

type test struct {
	source string
	tokens []token
}

func TestScanner(t *testing.T) {
	tests := []test{
		{"", []token{}},
		{"-", []token{{t: '-'}}},
		{"--[\n\n\r--]", []token{}},
		{"-- hello, world\n", []token{}},
		{"=", []token{{t: '='}}},
		{"==", []token{{t: tkEq}}},
		{"\"hello, world\"", []token{{t: tkString, s: "hello, world"}}},
		{"[[hello,\r\nworld]]", []token{{t: tkString, s: "hello,\n\nworld"}}},
		{".", []token{{t: '.'}}},
		{"..", []token{{t: tkConcat}}},
		{"...", []token{{t: tkDots}}},
		{".34", []token{{t: tkNumber, n: 0.34}}},
		{"_foo", []token{{t: tkName, s: "_foo"}}},
		{"3", []token{{t: tkNumber, n: float64(3)}}},
		{"3.0", []token{{t: tkNumber, n: 3.0}}},
		{"3.1416", []token{{t: tkNumber, n: 3.1416}}},
		{"314.16e-2", []token{{t: tkNumber, n: 3.1416}}},
		{"0.31416E1", []token{{t: tkNumber, n: 3.1416}}},
		{"0xff", []token{{t: tkNumber, n: float64(0xff)}}},
		{"0x0.1E", []token{{t: tkNumber, n: 0.1171875}}},
		{"0xA23p-4", []token{{t: tkNumber, n: 162.1875}}},
		{"0X1.921FB54442D18P+1", []token{{t: tkNumber, n: 3.141592653589793}}},
		{"  -0xa  ", []token{{t: '-'}, {t: tkNumber, n: 10.0}}},
	}
	for i, v := range tests {
		testScanner(t, i, v.source, v.tokens)
	}
}

func testScanner(t *testing.T, n int, source string, tokens []token) {
	s := scanner{r: strings.NewReader(source)}
	for i, expected := range tokens {
		if result := s.scan(); result != expected {
			t.Errorf("[%d] expected token %s but found %s at %d", n, expected, result, i)
		}
	}
	expected := token{t: tkEOS}
	if result := s.scan(); result != expected {
		t.Errorf("[%d] expected token %s but found %s", n, expected, result)
	}
}

func (t token) String() string {
	tok := string(t.t)
	if tkAnd <= t.t && t.t <= tkString {
		tok = tokens[t.t-firstReserved]
	}
	return fmt.Sprintf("{t:%s, n:%f, s:%q}", tok, t.n, t.s)
}
