[![Build Status](https://circleci.com/gh/Shopify/go-lua.png?circle-token=997f951c602c0c63a263eba92975428a49ee4c2e)](https://circleci.com/gh/Shopify/go-lua)

A Lua VM in pure Go
===================

go-lua is a port of the Lua 5.2 VM to pure Go. It is compatible with binary files dumped by ```luac```, from the [Lua reference implementation](http://www.lua.org/).

The motivation is to enable simple scripting of Go applications. Two immediate targets are stored procedures in [etcd](https://github.com/coreos/etcd) and flows in [Gonan](https://github.com/csfrancis/gonan).

Status
------

The core VM and compiler has been ported and tested. The compiler is able to correctly process all Lua source files from the [Lua test suite](http://www.lua.org/tests/5.2/). The VM has been tested to correctly execute over a third of the Lua test cases.

Most core Lua libraries are at least partially implemented. Prominent exceptions are regular expressions, coroutines and `string.dump`.

Weak reference tables are not and will not be supported. go-lua uses the Go heap for Lua objects, and Go does not support weak references.

Lessons
-------

`defer` in Go has a non-zero cost, and should not be used for normal control flow in performance-sensitive code paths.

The `gc` Go compiler does not compile (large) `switch` statements to jump tables. It instead compiles to a binary search ending in a `if-else if` cascade (when less than 4 cases).

The `gc` Go compiler performs very limited function call inlining. Inlining depends on the complexity of the callee function. In particular, function-call nodes in the callee function appear to have infinite cost, as do calls to panic and most calls into the language runtime (e.g. interface assertions). On the upside, function inlining is performed across files and sometimes across package boundaries.

The `gc` Go compiler compiles type switches as cascading `if-else if` tests, with repeated calls to a runtime type assertion function.

There doesn't appear to be a convenient way to avoid duplicated hash calculation for a check & set of a map entry.
