A Lua VM in pure Go
===================

go-lua is (intended to be) a Lua 5.2 VM implemented in pure Go. It is compatible with binary files dumped by ```luac```, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. Two immediate targets are stored procedures in [etcd](https://github.com/coreos/etcd) and flows in [Gonan](https://github.com/csfrancis/gonan).

Status
------

It is able to “undump” Lua binaries, such as ```checktable.bin``` compiled from the [Lua test suite](http://www.lua.org/tests/5.2/).

It can execute basic recursive functions, like ```fib```, tail-recursive functions & loops.

Tasks
-----

- [ ] Support calls to Go functions
- [ ] Implement Lua co-routines as goroutines
- [ ] Implement a compiler
- [ ] Replace brittle tests using ```luac``` compiled binaries with Lua source
- [ ] Implement the Lua core library
- [ ] Pass all tests from the [Lua test suite](http://www.lua.org/tests/5.2/) that are *not* specific to the C API