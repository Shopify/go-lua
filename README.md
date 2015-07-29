[![Build Status](https://circleci.com/gh/Shopify/go-lua.png?circle-token=997f951c602c0c63a263eba92975428a49ee4c2e)](https://circleci.com/gh/Shopify/go-lua)
[![GoDoc](https://godoc.org/github.com/Shopify/go-lua?status.png)](https://godoc.org/github.com/Shopify/go-lua)

A Lua VM in pure Go
===================

go-lua is a port of the Lua 5.2 VM to pure Go. It is compatible with binary files dumped by `luac`, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. For example, it is used to describe flows in [Shopify's](http://www.shopify.com/) load generation tool, Genghis.

Usage
-----

go-lua is intended to be used as a Go package. It does not include a command to run the interpreter. To start using the library, run:
```sh
go get github.com/Shopify/go-lua
```

To develop & test go-lua, you'll also need the [lua-tests](https://github.com/Shopify/lua-tests) submodule checked out:
```sh
git submodule update --init
```

You can then develop with the usual Go commands, e.g.:
```sh
go build
go test -cover
```

A simple example that loads & runs a Lua script is:
```go
package main

import "github.com/Shopify/go-lua"

func main() {
  l := lua.NewState()
  lua.OpenLibraries(l)
  if err := lua.DoFile(l, "hello.lua"); err != nil {
    panic(err)
  }
}
```

Status
------

go-lua has been used in production in Shopify's load generation tool, Genghis, since May 2014, and is also part of Shopify's resiliency tooling.

The core VM and compiler has been ported and tested. The compiler is able to correctly process all Lua source files from the [Lua test suite](https://github.com/Shopify/lua-tests). The VM has been tested to correctly execute over a third of the Lua test cases.

Most core Lua libraries are at least partially implemented. Prominent exceptions are regular expressions, coroutines and `string.dump`.

Weak reference tables are not and will not be supported. go-lua uses the Go heap for Lua objects, and Go does not support weak references.

Benchmarks
----------

Benchmark results shown here are taken from a Mid 2012 MacBook Pro Retina with a 2.6 GHz Core i7 CPU running OS X 10.10.2, go 1.4.2 and Lua 5.2.2.

The Fibonacci function can be written a few different ways to evaluate different performance characteristics of a language interpreter. The simplest way is as a recursive function:
```lua
  function fib(n)
    if n == 0 then
      return 0
    elseif n == 1 then
      return 1
    end
    return fib(n-1) + fib(n-2)
  end
```

This exercises the call stack implementation. When computing `fib(35)`, go-lua is about 6x slower than the C Lua interpreter. [Gopher-lua](https://github.com/yuin/gopher-lua) is about 20% faster than go-lua. Much of the performance difference between go-lua and gopher-lua comes from the inclusion of debug hooks in go-lua. The remainder is due to the call stack implementation - go-lua heap-allocates Lua stack frames with a separately allocated variant struct, as outlined above. Although it caches recently used stack frames, it is outperformed by the simpler statically allocated call stacks in gopher-lua.
```
  $ time lua fibr.lua
  real  0m2.807s
  user  0m2.795s
  sys   0m0.006s
  
  $ time glua fibr.lua
  real  0m14.528s
  user  0m14.513s
  sys   0m0.031s
  
  $ time go-lua fibr.lua
  real  0m17.411s
  user  0m17.514s
  sys   0m1.287s
```

The recursive Fibonacci function can be transformed into a tail-recursive variant:
```lua
  function fibt(n0, n1, c)
    if c == 0 then
      return n0
    else if c == 1 then
      return n1
    end
    return fibt(n1, n0+n1, c-1)
  end
  
  function fib(n)
    fibt(0, 1, n)
  end
```

The Lua interpreter detects and optimizes tail calls. This exhibits similar relative performance between the 3 interpreters, though gopher-lua edges ahead a little due to its simpler stack model and reduced bookkeeping.
```
  $ time lua fibt.lua
  real  0m0.099s
  user  0m0.096s
  sys   0m0.002s

  $ time glua fibt.lua
  real  0m0.489s
  user  0m0.484s
  sys   0m0.005s

  $ time go-lua fibt.lua
  real  0m0.607s
  user  0m0.610s
  sys   0m0.068s
```

Finally, we can write an explicitly iterative implementation:
```lua
  function fib(n)
    if n == 0 then
      return 0
    else if n == 1 then
      return 1
    end
    local n0, n1 = 0, 1
    for i = n, 2, -1 do
      local tmp = n0 + n1
      n0 = n1
      n1 = tmp
    end
    return n1
  end
```

This exercises more of the bytecode interpreter’s inner loop. Here we see the performance impact of Go’s `switch` implementation. Both go-lua and gopher-lua are an order of magnitude slower than the C Lua interpreter.
```
  $ time lua fibi.lua
  real  0m0.023s
  user  0m0.020s
  sys   0m0.003s

  $ time glua fibi.lua
  real  0m0.242s
  user  0m0.235s
  sys   0m0.005s

  $ time go-lua fibi.lua
  real  0m0.242s
  user  0m0.240s
  sys   0m0.028s
```

License
-------

go-lua is licensed under the [MIT license](https://github.com/Shopify/go-lua/blob/master/LICENSE.md).
