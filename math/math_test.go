package math

import (
	"github.com/Shopify/go-lua"
	"github.com/Shopify/go-lua/base"
	"testing"
)

func TestMath(t *testing.T) {
	lua.Call(lua.BinaryTest(t, "math.bin", lua.RegistryFunction{"_G", base.Open}, lua.RegistryFunction{"math", Open}), 0, 0)
}
