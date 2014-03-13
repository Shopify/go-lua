package lua_test

import (
	"github.com/Shopify/go-lua"
	"testing"
)

func TestCanRemoveNilObjectFromStack(t *testing.T) {

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("failed to remove `nil`, %v", r)
		}
	}()

	l := lua.NewState()

	lua.PushString(l, "hello")
	lua.Remove(l, -1)

	lua.PushNil(l)
	lua.Remove(l, -1)
}
