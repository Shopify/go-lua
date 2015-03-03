package main

import (
	"flag"
	"github.com/Shopify/go-lua"
)

func main() {
	flag.Parse()
	fileName := flag.Arg(0)
	l := lua.NewState()
	lua.OpenLibraries(l)
	if err := lua.LoadFile(l, fileName, ""); err != nil {
		panic(err)
	}
	if err := l.ProtectedCall(0, 0, 0); err != nil {
		panic(err)
	}
}
