package lua

import (
	"testing"
)

func TestCanRemoveNilObjectFromStack(t *testing.T) {

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("failed to remove `nil`, %v", r)
		}
	}()

	l := NewState()

	PushString(l, "hello")
	Remove(l, -1)

	PushNil(l)
	Remove(l, -1)
}
