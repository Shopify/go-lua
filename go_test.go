package lua

// Test assumptions about how Go works

import "testing"

func TestStringCompare(t *testing.T) {
	s1 := "hello\x00world"
	s2 := "hello\x00sweet"
	if s1 <= s2 {
		t.Error("s1 <= s2")
	}
}

func TestStringLength(t *testing.T) {
	s := "hello\x00world"
	if len(s) != 11 {
		t.Error("go doesn't count embedded nulls in string length")
	}
}

func TestReslicing(t *testing.T) {
	a := [5]int{0, 1, 2, 3, 4}
	s := a[:0]
	if cap(s) != cap(a) {
		t.Error("cap(s) != cap(a)")
	}
	if len(s) != 0 {
		t.Error("len(s) != 0")
	}
	s = a[1:3]
	if cap(s) == len(s) {
		t.Error("cap(s) == len(s)")
	}
	s = s[:cap(s)]
	if cap(s) != len(s) {
		t.Error("cap(s) != len(s)")
	}
}
