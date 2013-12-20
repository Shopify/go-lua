package lua

// Test assumptions about how Go works

import (
	"testing"
)

func TestStringCompare(t *testing.T) {
	s1 := "hello\x00world"
	s2 := "hello\x00sweet"
	if s1 <= s2 {
		t.Error("s1 <= s2")
	}
}

/* go test */
