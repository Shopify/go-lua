package lua

import "testing"

func TestCloseUnlinksAdjacentClosedUpValues(t *testing.T) {
	l := NewState()
	l.stack[1] = "kept open"
	l.stack[2] = "closed first"
	l.stack[3] = "closed second"

	kept := l.newUpValueAt(1)
	closedFirst := l.newUpValueAt(2)
	closedSecond := l.newUpValueAt(3)

	l.close(2)

	if got := closedFirst.value(); got != "closed first" {
		t.Errorf("first closed upvalue = %v, want %q", got, "closed first")
	}
	if got := closedSecond.value(); got != "closed second" {
		t.Errorf("second closed upvalue = %v, want %q", got, "closed second")
	}
	if !kept.isInStackAt(1) {
		t.Fatal("upvalue below the closing level was closed")
	}
	if l.upValues == nil || l.upValues.upValue != kept || l.upValues.next != nil {
		t.Error("closed upvalues remain in the open upvalue list")
	}
}
