package lua

import (
	"fmt"
	"testing"
)

func TestPushFStringPointer(t *testing.T) {
	l := NewState()
	l.PushFString("%p %s", l, "test")

	expected := fmt.Sprintf("%p %s", l, "test")
	actual := CheckString(l, -1)
	if expected != actual {
		t.Errorf("PushFString, expected \"%s\" but found \"%s\"", expected, actual)
	}
}

func TestToBooleanOutOfRange(t *testing.T) {
	l := NewState()
	l.SetTop(0)
	l.PushBoolean(false)
	l.PushBoolean(true)

	for i, want := range []bool{false, true, false, false} {
		idx := 1 + i
		if got := l.ToBoolean(idx); got != want {
			t.Errorf("l.ToBoolean(%d) = %t; want %t", idx, got, want)
		}
	}
}
