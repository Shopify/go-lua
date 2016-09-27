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
