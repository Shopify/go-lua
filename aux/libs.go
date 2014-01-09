package aux

import (
	"github.com/Shopify/go-lua"
	"github.com/Shopify/go-lua/base"
	"github.com/Shopify/go-lua/math"
)

func OpenLibraries(l lua.State) {
	libs := []lua.RegistryFunction{
		{"_G", base.Open},
		{"math", math.Open},
	}
	for _, lib := range libs {
		lua.Require(l, lib.Name, lib.Function, true)
		l.Pop(1)
	}
	// TODO support preloaded libraries
}
