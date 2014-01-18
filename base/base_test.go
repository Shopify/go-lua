package base

import (
	"github.com/Shopify/go-lua"
	"testing"
)

func TestBase(t *testing.T) {
	lua.Call(lua.BinaryTest(t, "base.bin", lua.RegistryFunction{"_G", Open}), 0, 0)
}
