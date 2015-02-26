[![Build Status](https://circleci.com/gh/Shopify/go-lua.png?circle-token=997f951c602c0c63a263eba92975428a49ee4c2e)](https://circleci.com/gh/Shopify/go-lua)

A Lua VM in pure Go
===================

go-lua is a port of the Lua 5.2 VM to pure Go. It is compatible with binary files dumped by `luac`, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. For example, it is used to describe flows in [Shopify's](http://www.shopify.com/) load generation tool, Genghis.

Status
------

go-lua has been used in production in Shopify's load generation tool, Genghis, since May 2014, and is also part of Shopify's resiliency tooling.

The core VM and compiler has been ported and tested. The compiler is able to correctly process all Lua source files from the [Lua test suite](https://github.com/Shopify/lua-tests). The VM has been tested to correctly execute over a third of the Lua test cases.

Most core Lua libraries are at least partially implemented. Prominent exceptions are regular expressions, coroutines and `string.dump`.

Weak reference tables are not and will not be supported. go-lua uses the Go heap for Lua objects, and Go does not support weak references.

License
-------

go-lua is licensed under the [MIT license](https://github.com/Shopify/go-lua/blob/master/LICENSE.md).
