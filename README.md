[![Build Status](https://circleci.com/gh/Shopify/go-lua.png?circle-token=997f951c602c0c63a263eba92975428a49ee4c2e)](https://circleci.com/gh/Shopify/go-lua)

A Lua VM in pure Go
===================

go-lua is a port of the Lua 5.2 VM to pure Go. It is compatible with binary files dumped by ```luac```, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. Two immediate targets are stored procedures in [etcd](https://github.com/coreos/etcd) and flows in [Gonan](https://github.com/csfrancis/gonan).

Status
------

The core VM and compiler has been ported and tested. The compiler is able to correctly process all Lua source files from the [Lua test suite](http://www.lua.org/tests/5.2/). The VM has been tested to correctly execute `bitwise.lua`, `math.lua` and most of `goto.lua`.

The following Lua libraries are implemented: `base`, `bit32`, `math`, along with parts of `string` and `table`.
