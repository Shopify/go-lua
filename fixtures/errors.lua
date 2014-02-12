print("testing errors")

local debug = require"debug"

-- avoid problems with 'strict' module (which may generate other error messages)
local mt = getmetatable(_G) or {}
local oldmm = mt.__index
mt.__index = nil

function doit (s)
  local f, msg = load(s)
  if f == nil then return msg end
  local cond, msg = pcall(f)
  return (not cond) and msg
end


function checkmessage (prog, msg)
  local m = doit(prog)
  assert(string.find(m, msg, 1, true))
end

function checksyntax (prog, extra, token, line)
  local msg = doit(prog)
  if not string.find(token, "^<%a") and not string.find(token, "^char%(")
    then token = "'"..token.."'" end
  token = string.gsub(token, "(%p)", "%%%1")
  local pt = string.format([[^%%[string ".*"%%]:%d: .- near %s$]],
                           line, token)
  assert(string.find(msg, pt))
  assert(string.find(msg, msg, 1, true))
end


-- test error message with no extra info
assert(doit("error('hi', 0)") == 'hi')

-- test error message with no info
assert(doit("error()") == nil)


-- test common errors/errors that crashed in the past
if not _no32 then
  assert(doit("table.unpack({}, 1, n=2^30)"))
end
assert(doit("a=math.sin()"))
assert(not doit("tostring(1)") and doit("tostring()"))
assert(doit"tonumber()")
assert(doit"repeat until 1; a")
assert(doit"return;;")
assert(doit"assert(false)")
assert(doit"assert(nil)")
assert(doit("function a (... , ...) end"))
assert(doit("function a (, ...) end"))
assert(doit("local t={}; t = t[#t] + 1"))

checksyntax([[
  local a = {4

]], "'}' expected (to close '{' at line 1)", "<eof>", 3)


-- tests for better error messages

checkmessage("a=1; bbbb=2; a=math.sin(3)+bbbb(3)", "global 'bbbb'")
checkmessage("a=1; local a,bbbb=2,3; a = math.sin(1) and bbbb(3)",
       "local 'bbbb'")
checkmessage("a={}; do local a=1 end a:bbbb(3)", "method 'bbbb'")
checkmessage("local a={}; a.bbbb(3)", "field 'bbbb'")
assert(not string.find(doit"a={13}; local bbbb=1; a[bbbb](3)", "'bbbb'"))
checkmessage("a={13}; local bbbb=1; a[bbbb](3)", "number")
checkmessage("a=(1)..{}", "a table value")

aaa = nil
checkmessage("aaa.bbb:ddd(9)", "global 'aaa'")
checkmessage("local aaa={bbb=1}; aaa.bbb:ddd(9)", "field 'bbb'")
checkmessage("local aaa={bbb={}}; aaa.bbb:ddd(9)", "method 'ddd'")
checkmessage("local a,b,c; (function () a = b+1 end)()", "upvalue 'b'")
assert(not doit"local aaa={bbb={ddd=next}}; aaa.bbb:ddd(nil)")

checkmessage("local _ENV = {x={}}; a = a + 1", "global 'a'")

checkmessage("b=1; local aaa='a'; x=aaa+b", "local 'aaa'")
checkmessage("aaa={}; x=3/aaa", "global 'aaa'")
checkmessage("aaa='2'; b=nil;x=aaa*b", "global 'b'")
checkmessage("aaa={}; x=-aaa", "global 'aaa'")
assert(not string.find(doit"aaa={}; x=(aaa or aaa)+(aaa and aaa)", "'aaa'"))
assert(not string.find(doit"aaa={}; (aaa or aaa)()", "'aaa'"))

checkmessage("print(print < 10)", "function")
checkmessage("print(print < print)", "two function")


-- passing light userdata instead of full userdata
_G.D = debug
checkmessage([[
  -- create light udata
  local x = D.upvalueid(function () return debug end, 1)
  D.setuservalue(x, {})
]], "light userdata")
_G.D = nil


-- global functions
checkmessage("(io.write or print){}", "io.write")
checkmessage("(collectgarbage or print){}", "collectgarbage")

-- tests for field accesses after RK limit
local t = {}
for i = 1, 1000 do
  t[i] = "a = x" .. i
end
local s = table.concat(t, "; ")
t = nil
checkmessage(s.."; a = bbb + 1", "global 'bbb'")
checkmessage("local _ENV=_ENV;"..s.."; a = bbb + 1", "global 'bbb'")
checkmessage(s.."; local t = {}; a = t.bbb + 1", "field 'bbb'")
checkmessage(s.."; local t = {}; t:bbb()", "method 'bbb'")

checkmessage([[aaa=9
repeat until 3==3
local x=math.sin(math.cos(3))
if math.sin(1) == x then return math.sin(1) end   -- tail call
local a,b = 1, {
  {x='a'..'b'..'c', y='b', z=x},
  {1,2,3,4,5} or 3+3<=3+3,
  3+1>3+1,
  {d = x and aaa[x or y]}}
]], "global 'aaa'")

checkmessage([[
local x,y = {},1
if math.sin(1) == 0 then return 3 end    -- return
x.a()]], "field 'a'")

checkmessage([[
prefix = nil
insert = nil
while 1 do
  local a
  if nil then break end
  insert(prefix, a)
end]], "global 'insert'")

checkmessage([[  -- tail call
  return math.sin("a")
]], "'sin'")

checkmessage([[collectgarbage("nooption")]], "invalid option")

checkmessage([[x = print .. "a"]], "concatenate")

checkmessage("getmetatable(io.stdin).__gc()", "no value")

checkmessage([[
local Var
local function main()
  NoSuchName (function() Var=0 end)
end
main()
]], "global 'NoSuchName'")
print'+'

a = {}; setmetatable(a, {__index = string})
checkmessage("a:sub()", "bad self")
checkmessage("string.sub('a', {})", "#2")
checkmessage("('a'):sub{}", "#1")

checkmessage("table.sort({1,2,3}, table.sort)", "'table.sort'")
-- next message may be 'setmetatable' or '_G.setmetatable'
checkmessage("string.gsub('s', 's', setmetatable)", "setmetatable'")

-- tests for errors in coroutines

function f (n)
  local c = coroutine.create(f)
  local a,b = coroutine.resume(c)
  return b
end
assert(string.find(f(), "C stack overflow"))

checkmessage("coroutine.yield()", "outside a coroutine")

f1 = function () table.sort({1,2,3}, coroutine.yield) end
f = coroutine.wrap(function () return pcall(f1) end)
assert(string.find(select(2, f()), "yield across"))


-- testing size of 'source' info; size of buffer for that info is
-- LUA_IDSIZE, declared as 60 in luaconf. Get one position for '\0'.
idsize = 60 - 1
local function checksize (source)
  -- syntax error
  local _, msg = load("x", source)
  msg = string.match(msg, "^([^:]*):")   -- get source (1st part before ':')
  assert(msg:len() <= idsize)
end

for i = 60 - 10, 60 + 10 do   -- check border cases around 60
  checksize("@" .. string.rep("x", i))   -- file names
  checksize(string.rep("x", i - 10))     -- string sources
  checksize("=" .. string.rep("x", i))   -- exact sources
end


-- testing line error

local function lineerror (s, l)
  local err,msg = pcall(load(s))
  local line = string.match(msg, ":(%d+):")
  assert((line and line+0) == l)
end

lineerror("local a\n for i=1,'a' do \n print(i) \n end", 2)
lineerror("\n local a \n for k,v in 3 \n do \n print(k) \n end", 3)
lineerror("\n\n for k,v in \n 3 \n do \n print(k) \n end", 4)
lineerror("function a.x.y ()\na=a+1\nend", 1)

lineerror("a = \na\n+\n{}", 3)
lineerror("a = \n3\n+\n(\n4\n/\nprint)", 6)
lineerror("a = \nprint\n+\n(\n4\n/\n7)", 3)

lineerror("a\n=\n-\n\nprint\n;", 3)

lineerror([[
a
(
23)
]], 1)

lineerror([[
local a = {x = 13}
a
.
x
(
23
)
]], 2)

lineerror([[
local a = {x = 13}
a
.
x
(
23 + a
)
]], 6)

local p = [[
function g() f() end
function f(x) error('a', X) end
g()
]]
X=3;lineerror((p), 3)
X=0;lineerror((p), nil)
X=1;lineerror((p), 2)
X=2;lineerror((p), 1)


if not _soft then
  -- several tests that exaust the Lua stack
  C = 0
  local l = debug.getinfo(1, "l").currentline; function y () C=C+1; y() end

  local function checkstackmessage (m)
    return (string.find(m, "^.-:%d+: stack overflow"))
  end
  -- repeated stack overflows (to check stack recovery)
  assert(checkstackmessage(doit('y()')))
  print('+')
  assert(checkstackmessage(doit('y()')))
  print('+')
  assert(checkstackmessage(doit('y()')))
  print('+')


  -- error lines in stack overflow
  C = 0
  local l1
  local function g(x)
    l1 = debug.getinfo(x, "l").currentline; y()
  end
  local _, stackmsg = xpcall(g, debug.traceback, 1)
  print('+')
  local stack = {}
  for line in string.gmatch(stackmsg, "[^\n]*") do
    local curr = string.match(line, ":(%d+):")
    if curr then table.insert(stack, tonumber(curr)) end
  end
  local i=1
  while stack[i] ~= l1 do
    assert(stack[i] == l)
    i = i+1
  end
  assert(i > 15)


  -- error in error handling
  local res, msg = xpcall(error, error)
  assert(not res and type(msg) == 'string')
  print('+')

  local function f (x)
    if x==0 then error('a\n')
    else
      local aux = function () return f(x-1) end
      local a,b = xpcall(aux, aux)
      return a,b
    end
  end
  f(3)

  local function loop (x,y,z) return 1 + loop(x, y, z) end
 
  local res, msg = xpcall(loop, function (m)
    assert(string.find(m, "stack overflow"))
    local res, msg = pcall(loop)
    assert(string.find(msg, "error handling"))
    assert(math.sin(0) == 0)
    return 15
  end)
  assert(msg == 15)

  res, msg = pcall(function ()
    for i = 999900, 1000000, 1 do table.unpack({}, 1, i) end
  end)
  assert(string.find(msg, "too many results"))

end


-- non string messages
function f() error{msg='x'} end
res, msg = xpcall(f, function (r) return {msg=r.msg..'y'} end)
assert(msg.msg == 'xy')

-- xpcall with arguments
a, b, c = xpcall(string.find, error, "alo", "al")
assert(a and b == 1 and c == 2)
a, b, c = xpcall(string.find, function (x) return {} end, true, "al")
assert(not a and type(b) == "table" and c == nil)

print('+')
checksyntax("syntax error", "", "error", 1)
checksyntax("1.000", "", "1.000", 1)
checksyntax("[[a]]", "", "[[a]]", 1)
checksyntax("'aa'", "", "'aa'", 1)

-- test 255 as first char in a chunk
checksyntax("\255a = 1", "", "char(255)", 1)

doit('I = load("a=9+"); a=3')
assert(a==3 and I == nil)
print('+')

lim = 1000
if _soft then lim = 100 end
for i=1,lim do
  doit('a = ')
  doit('a = 4+nil')
end


-- testing syntax limits
local function testrep (init, rep)
  local s = "local a; "..init .. string.rep(rep, 400)
  local a,b = load(s)
  assert(not a and string.find(b, "levels"))
end
testrep("a=", "{")
testrep("a=", "(")
testrep("", "a(")
testrep("", "do ")
testrep("", "while a do ")
testrep("", "if a then else ")
testrep("", "function foo () ")
testrep("a=", "a..")
testrep("a=", "a^")

local s = ("a,"):rep(200).."a=nil"
local a,b = load(s)
assert(not a and string.find(b, "levels"))


-- testing other limits
-- upvalues
local lim = 127
local  s = "local function fooA ()\n  local "
for j = 1,lim do
  s = s.."a"..j..", "
end
s = s.."b,c\n"
s = s.."local function fooB ()\n  local "
for j = 1,lim do
  s = s.."b"..j..", "
end
s = s.."b\n"
s = s.."function fooC () return b+c"
local c = 1+2
for j = 1,lim do
  s = s.."+a"..j.."+b"..j
  c = c + 2
end
s = s.."\nend  end end"
local a,b = load(s)
assert(c > 255 and string.find(b, "too many upvalues") and
       string.find(b, "line 5"))

-- local variables
s = "\nfunction foo ()\n  local "
for j = 1,300 do
  s = s.."a"..j..", "
end
s = s.."b\n"
local a,b = load(s)
assert(string.find(b, "line 2"))

mt.__index = oldmm

print('OK')
