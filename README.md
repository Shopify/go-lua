[![Build Status](https://circleci.com/gh/Shopify/go-lua.png?circle-token=997f951c602c0c63a263eba92975428a49ee4c2e)](https://circleci.com/gh/Shopify/go-lua)

A Lua VM in pure Go
===================

go-lua is (intended to be) a Lua 5.2 VM implemented in pure Go. It is compatible with binary files dumped by ```luac```, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. Two immediate targets are stored procedures in [etcd](https://github.com/coreos/etcd) and flows in [Gonan](https://github.com/csfrancis/gonan).

Status
------

It is able to “undump” Lua binaries, such as ```checktable.bin``` compiled from the [Lua test suite](http://www.lua.org/tests/5.2/).

It can execute basic recursive functions, like ```fib```, tail-recursive functions & loops.
