-- The tests for 'require' assume some specific directories and libraries;
-- better to avoid them in generic machines

if not _port then --[

print "testing require"

assert(require"string" == string)
assert(require"math" == math)
assert(require"table" == table)
assert(require"io" == io)
assert(require"os" == os)
assert(require"coroutine" == coroutine)

assert(type(package.path) == "string")
assert(type(package.cpath) == "string")
assert(type(package.loaded) == "table")
assert(type(package.preload) == "table")

assert(type(package.config) == "string")
print("package config: "..string.gsub(package.config, "\n", "|"))

do
  -- create a path with 'max' templates,
  -- each with 1-10 repetitions of '?'
  local max = 2000
  local t = {}
  for i = 1,max do t[i] = string.rep("?", i%10 + 1) end
  t[#t + 1] = ";"    -- empty template
  local path = table.concat(t, ";")
  -- use that path in a search
  local s, err = package.searchpath("xuxu", path)
  -- search fails; check that message has an occurence of
  -- '??????????' with ? replaced by xuxu and at least 'max' lines
  assert(not s and
         string.find(err, string.rep("xuxu", 10)) and
         #string.gsub(err, "[^\n]", "") >= max)
  -- path with one very long template
  local path = string.rep("?", max)
  local s, err = package.searchpath("xuxu", path)
  assert(not s and string.find(err, string.rep('xuxu', max)))
end

do
  local oldpath = package.path
  package.path = {}
  local s, err = pcall(require, "no-such-file")
  assert(not s and string.find(err, "package.path"))
  package.path = oldpath
end

print('+')

-- auxiliary directory with C modules and temporary files
local DIR = "libs/"

-- prepend DIR to a name
local function D (x) return DIR .. x end


local function createfiles (files, preextras, posextras)
  for n,c in pairs(files) do
    io.output(D(n))
    io.write(string.format(preextras, n))
    io.write(c)
    io.write(string.format(posextras, n))
    io.close(io.output())
  end
end

function removefiles (files)
  for n in pairs(files) do
    os.remove(D(n))
  end
end

local files = {
  ["names.lua"] = "do return {...} end\n",
  ["err.lua"] = "B = 15; a = a + 1;",
  ["A.lua"] = "",
  ["B.lua"] = "assert(...=='B');require 'A'",
  ["A.lc"] = "",
  ["A"] = "",
  ["L"] = "",
  ["XXxX"] = "",
  ["C.lua"] = "package.loaded[...] = 25; require'C'"
}

AA = nil
local extras = [[
NAME = '%s'
REQUIRED = ...
return AA]]

createfiles(files, "", extras)

-- testing explicit "dir" separator in 'searchpath'
assert(package.searchpath("C.lua", D"?", "", "") == D"C.lua")
assert(package.searchpath("C.lua", D"?", ".", ".") == D"C.lua")
assert(package.searchpath("--x-", D"?", "-", "X") == D"XXxX")
assert(package.searchpath("---xX", D"?", "---", "XX") == D"XXxX")
assert(package.searchpath(D"C.lua", "?", "/") == D"C.lua")
assert(package.searchpath(".\\C.lua", D"?", "\\") == D"./C.lua")

local oldpath = package.path

package.path = string.gsub("D/?.lua;D/?.lc;D/?;D/??x?;D/L", "D/", DIR)

local try = function (p, n, r)
  NAME = nil
  local rr = require(p)
  assert(NAME == n)
  assert(REQUIRED == p)
  assert(rr == r)
end

a = require"names"
assert(a[1] == "names" and a[2] == D"names.lua")

_G.a = nil
assert(not pcall(require, "err"))
assert(B == 15)

assert(package.searchpath("C", package.path) == D"C.lua")
assert(require"C" == 25)
assert(require"C" == 25)
AA = nil
try('B', 'B.lua', true)
assert(package.loaded.B)
assert(require"B" == true)
assert(package.loaded.A)
assert(require"C" == 25)
package.loaded.A = nil
try('B', nil, true)   -- should not reload package
try('A', 'A.lua', true)
package.loaded.A = nil
os.remove(D'A.lua')
AA = {}
try('A', 'A.lc', AA)  -- now must find second option
assert(package.searchpath("A", package.path) == D"A.lc")
assert(require("A") == AA)
AA = false
try('K', 'L', false)     -- default option
try('K', 'L', false)     -- default option (should reload it)
assert(rawget(_G, "_REQUIREDNAME") == nil)

AA = "x"
try("X", "XXxX", AA)


removefiles(files)


-- testing require of sub-packages

local _G = _G

package.path = string.gsub("D/?.lua;D/?/init.lua", "D/", DIR)

files = {
  ["P1/init.lua"] = "AA = 10",
  ["P1/xuxu.lua"] = "AA = 20",
}

createfiles(files, "_ENV = {}\n", "\nreturn _ENV\n")
AA = 0

local m = assert(require"P1")
assert(AA == 0 and m.AA == 10)
assert(require"P1" == m)
assert(require"P1" == m)

assert(package.searchpath("P1.xuxu", package.path) == D"P1/xuxu.lua")
m.xuxu = assert(require"P1.xuxu")
assert(AA == 0 and m.xuxu.AA == 20)
assert(require"P1.xuxu" == m.xuxu)
assert(require"P1.xuxu" == m.xuxu)
assert(require"P1" == m and m.AA == 10)


removefiles(files)


package.path = ""
assert(not pcall(require, "file_does_not_exist"))
package.path = "??\0?"
assert(not pcall(require, "file_does_not_exist1"))

package.path = oldpath

-- check 'require' error message
local fname = "file_does_not_exist2"
local m, err = pcall(require, fname)
for t in string.gmatch(package.path..";"..package.cpath, "[^;]+") do
  t = string.gsub(t, "?", fname)
  assert(string.find(err, t, 1, true))
end


local function import(...)
  local f = {...}
  return function (m)
    for i=1, #f do m[f[i]] = _G[f[i]] end
  end
end

-- cannot change environment of a C function
assert(not pcall(module, 'XUXU'))



-- testing require of C libraries


local p = ""   -- On Mac OS X, redefine this to "_"

-- check whether loadlib works in this system
local st, err, when = package.loadlib(D"lib1.so", "*")
if not st then
  local f, err, when = package.loadlib("donotexist", p.."xuxu")
  assert(not f and type(err) == "string" and when == "absent")
  ;(Message or print)('\a\n >>> cannot load dynamic library <<<\n\a')
  print(err, when)
else
  -- tests for loadlib
  local f = assert(package.loadlib(D"lib1.so", p.."onefunction"))
  local a, b = f(15, 25)
  assert(a == 25 and b == 15)

  f = assert(package.loadlib(D"lib1.so", p.."anotherfunc"))
  assert(f(10, 20) == "1020\n")

  -- check error messages
  local f, err, when = package.loadlib(D"lib1.so", p.."xuxu")
  assert(not f and type(err) == "string" and when == "init")
  f, err, when = package.loadlib("donotexist", p.."xuxu")
  assert(not f and type(err) == "string" and when == "open")

  -- symbols from 'lib1' must be visible to other libraries
  f = assert(package.loadlib(D"lib11.so", p.."luaopen_lib11"))
  assert(f() == "exported")

  -- test C modules with prefixes in names
  package.cpath = D"?.so"
  local lib2 = require"v-lib2"
  -- check correct access to global environment and correct
  -- parameters
  assert(_ENV.x == "v-lib2" and _ENV.y == D"v-lib2.so")
  assert(lib2.id("x") == "x")

  -- test C submodules
  local fs = require"lib1.sub"
  assert(_ENV.x == "lib1.sub" and _ENV.y == D"lib1.so")
  assert(fs.id(45) == 45)
end

_ENV = _G


-- testing preload

do
  local p = package
  package = {}
  p.preload.pl = function (...)
    local _ENV = {...}
    function xuxu (x) return x+20 end
    return _ENV
  end

  local pl = require"pl"
  assert(require"pl" == pl)
  assert(pl.xuxu(10) == 30)
  assert(pl[1] == "pl" and pl[2] == nil)

  package = p
  assert(type(package.path) == "string")
end

print('+')

end  --]

print("testing assignments, logical operators, and constructors")

local res, res2 = 27

a, b = 1, 2+3
assert(a==1 and b==5)
a={}
function f() return 10, 11, 12 end
a.x, b, a[1] = 1, 2, f()
assert(a.x==1 and b==2 and a[1]==10)
a[f()], b, a[f()+3] = f(), a, 'x'
assert(a[10] == 10 and b == a and a[13] == 'x')

do
  local f = function (n) local x = {}; for i=1,n do x[i]=i end;
                         return table.unpack(x) end;
  local a,b,c
  a,b = 0, f(1)
  assert(a == 0 and b == 1)
  A,b = 0, f(1)
  assert(A == 0 and b == 1)
  a,b,c = 0,5,f(4)
  assert(a==0 and b==5 and c==1)
  a,b,c = 0,5,f(0)
  assert(a==0 and b==5 and c==nil)
end

a, b, c, d = 1 and nil, 1 or nil, (1 and (nil or 1)), 6
assert(not a and b and c and d==6)

d = 20
a, b, c, d = f()
assert(a==10 and b==11 and c==12 and d==nil)
a,b = f(), 1, 2, 3, f()
assert(a==10 and b==1)

assert(a<b == false and a>b == true)
assert((10 and 2) == 2)
assert((10 or 2) == 10)
assert((10 or assert(nil)) == 10)
assert(not (nil and assert(nil)))
assert((nil or "alo") == "alo")
assert((nil and 10) == nil)
assert((false and 10) == false)
assert((true or 10) == true)
assert((false or 10) == 10)
assert(false ~= nil)
assert(nil ~= false)
assert(not nil == true)
assert(not not nil == false)
assert(not not 1 == true)
assert(not not a == true)
assert(not not (6 or nil) == true)
assert(not not (nil and 56) == false)
assert(not not (nil and true) == false)

assert({} ~= {})
print('+')

a = {}
a[true] = 20
a[false] = 10
assert(a[1<2] == 20 and a[1>2] == 10)

function f(a) return a end

local a = {}
for i=3000,-3000,-1 do a[i] = i; end
a[10e30] = "alo"; a[true] = 10; a[false] = 20
assert(a[10e30] == 'alo' and a[not 1] == 20 and a[10<20] == 10)
for i=3000,-3000,-1 do assert(a[i] == i); end
a[print] = assert
a[f] = print
a[a] = a
assert(a[a][a][a][a][print] == assert)
a[print](a[a[f]] == a[print])
assert(not pcall(function () local a = {}; a[nil] = 10 end))
assert(not pcall(function () local a = {[nil] = 10} end))
assert(a[nil] == nil)
a = nil

a = {10,9,8,7,6,5,4,3,2; [-3]='a', [f]=print, a='a', b='ab'}
a, a.x, a.y = a, a[-3]
assert(a[1]==10 and a[-3]==a.a and a[f]==print and a.x=='a' and not a.y)
a[1], f(a)[2], b, c = {['alo']=assert}, 10, a[1], a[f], 6, 10, 23, f(a), 2
a[1].alo(a[2]==10 and b==10 and c==print)

a[2^31] = 10; a[2^31+1] = 11; a[-2^31] = 12;
a[2^32] = 13; a[-2^32] = 14; a[2^32+1] = 15; a[10^33] = 16;

assert(a[2^31] == 10 and a[2^31+1] == 11 and a[-2^31] == 12 and
       a[2^32] == 13 and a[-2^32] == 14 and a[2^32+1] == 15 and
       a[10^33] == 16)

a = nil


-- test conflicts in multiple assignment
do
  local a,i,j,b
  a = {'a', 'b'}; i=1; j=2; b=a
  i, a[i], a, j, a[j], a[i+j] = j, i, i, b, j, i
  assert(i == 2 and b[1] == 1 and a == 1 and j == b and b[2] == 2 and
         b[3] == 1)
end

-- repeat test with upvalues
do
  local a,i,j,b
  a = {'a', 'b'}; i=1; j=2; b=a
  local function foo ()
    i, a[i], a, j, a[j], a[i+j] = j, i, i, b, j, i
  end
  foo()
  assert(i == 2 and b[1] == 1 and a == 1 and j == b and b[2] == 2 and
         b[3] == 1)
  local t = {}
  (function (a) t[a], a = 10, 20  end)(1);
  assert(t[1] == 10)
end

-- bug in 5.2 beta
local function foo ()
  local a
  return function ()
    local b
    a, b = 3, 14    -- local and upvalue have same index
    return a, b
  end
end

local a, b = foo()()
assert(a == 3 and b == 14)

print('OK')

return res

