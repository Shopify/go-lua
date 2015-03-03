package main

import (
	"flag"
	"github.com/Shopify/go-lua"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	flag.Parse()
	fileName := flag.Arg(0)
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	l := lua.NewState()
	lua.OpenLibraries(l)
	if err := lua.LoadFile(l, fileName, ""); err != nil {
		log.Fatal(err)
	}
	if err := l.ProtectedCall(0, 0, 0); err != nil {
		log.Fatal(err)
	}
}
