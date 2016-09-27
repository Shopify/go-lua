package lua

import (
	"regexp"
	"testing"
)

// https://github.com/Shopify/go-lua/pull/63
func TestPushFStringPointer(t *testing.T) {
	l := NewState()
	l.PushFString("%p %s", l, "test")

	actual := CheckString(l, -1)
	ok, err := regexp.MatchString("0x[0-9a-f]+ test", actual)
	if !ok {
		t.Error("regex did not match")
	} else if err != nil {
		t.Errorf("regex error: %s", err.Error())
	}
}
