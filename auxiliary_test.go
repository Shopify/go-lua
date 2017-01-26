package lua

import "testing"

func TestLoadFileSyntaxError(t *testing.T) {
	l := NewState()
	err := LoadFile(l, "fixtures/syntax_error.lua", "")
	if err != SyntaxError {
		t.Error("didn't return SyntaxError on file with syntax error")
	}
	if l.Top() != 1 {
		t.Error("didn't push anything to the stack")
	}
	if l.IsString(-1) != true {
		t.Error("didn't push a string to the stack")
	}
	estr, _ := l.ToString(-1)
	if estr != "fixtures/syntax_error.lua:4: syntax error near <eof>" {
		t.Error("didn't push the correct error string")
	}
}

func TestLoadStringSyntaxError(t *testing.T) {
	l := NewState()
	err := LoadString(l, "this_is_a_syntax_error")
	if err != SyntaxError {
		t.Error("didn't return SyntaxError on string with syntax error")
	}
	if l.Top() != 1 {
		t.Error("didn't push anything to the stack")
	}
	if l.IsString(-1) != true {
		t.Error("didn't push a string to the stack")
	}
	estr, _ := l.ToString(-1)
	if estr != "[string \"this_is_a_syntax_error\"]:1: syntax error near <eof>" {
		t.Error("didn't push the correct error string")
	}
}
