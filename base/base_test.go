package base

import (
	"github.com/Shopify/go-lua"
	"testing"
)

func TestBase(t *testing.T) {
	lua.BinaryTest(t, "base.bin", lua.RegistryFunction{"_G", Open}).Call(0, 0)
}
