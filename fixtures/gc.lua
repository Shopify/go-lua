print('testing garbage collection')

collectgarbage()

assert(collectgarbage("isrunning"))

local function gcinfo () return collectgarbage"count" * 1024 end


-- test weird parameters
do
  -- save original parameters
  local a = collectgarbage("setpause", 200)
  local b = collectgarbage("setstepmul", 200)
  local t = {0, 2, 10, 90, 500, 5000, 30000, 2^31 - 2}
  for i = 1, #t do
    local p = t[i]
    for j = 1, #t do
      local m = t[j]
      collectgarbage("setpause", p)
      collectgarbage("setstepmul", m)
      collectgarbage("step", 0)
      collectgarbage("step", 10000)
    end
  end
  -- restore original parameters
  collectgarbage("setpause", a)
  collectgarbage("setstepmul", b)
  collectgarbage()
end


_G["while"] = 234

limit = 5000


local function GC1 ()
  local u
  local b     -- must be declared after 'u' (to be above it in the stack)
  local finish = false
  u = setmetatable({}, {__gc = function () finish = true end})
  b = {34}
  repeat u = {} until finish
  assert(b[1] == 34)   -- 'u' was collected, but 'b' was not

  finish = false; local i = 1
  u = setmetatable({}, {__gc = function () finish = true end})
  repeat i = i + 1; u = i .. i until finish
  assert(b[1] == 34)   -- 'u' was collected, but 'b' was not

  finish = false
  u = setmetatable({}, {__gc = function () finish = true end})
  repeat local i; u = function () return i end until finish
  assert(b[1] == 34)   -- 'u' was collected, but 'b' was not
end

local function GC()  GC1(); GC1() end


contCreate = 0

print('tables')
while contCreate <= limit do
  local a = {}; a = nil
  contCreate = contCreate+1
end

a = "a"

contCreate = 0
print('strings')
while contCreate <= limit do
  a = contCreate .. "b";
  a = string.gsub(a, '(%d%d*)', string.upper)
  a = "a"
  contCreate = contCreate+1
end


contCreate = 0

a = {}

print('functions')
function a:test ()
  while contCreate <= limit do
    load(string.format("function temp(a) return 'a%d' end", contCreate))()
    assert(temp() == string.format('a%d', contCreate))
    contCreate = contCreate+1
  end
end

a:test()

-- collection of functions without locals, globals, etc.
do local f = function () end end


print("functions with errors")
prog = [[
do
  a = 10;
  function foo(x,y)
    a = sin(a+0.456-0.23e-12);
    return function (z) return sin(%x+z) end
  end
  local x = function (w) a=a+w; end
end
]]
do
  local step = 1
  if _soft then step = 13 end
  for i=1, string.len(prog), step do
    for j=i, string.len(prog), step do
      pcall(load(string.sub(prog, i, j)))
    end
  end
end

foo = nil
print('long strings')
x = "01234567890123456789012345678901234567890123456789012345678901234567890123456789"
assert(string.len(x)==80)
s = ''
n = 0
k = 300
while n < k do s = s..x; n=n+1; j=tostring(n)  end
assert(string.len(s) == k*80)
s = string.sub(s, 1, 20000)
s, i = string.gsub(s, '(%d%d%d%d)', math.sin)
assert(i==20000/4)
s = nil
x = nil

assert(_G["while"] == 234)

local k,b = collectgarbage("count")
assert(k*1024 == math.floor(k)*1024 + b)

print("steps")

local bytes = gcinfo()
while 1 do
  local nbytes = gcinfo()
  if nbytes < bytes then break end   -- run until gc
  bytes = nbytes
  a = {}
end

print("steps (2)")

local function dosteps (siz)
  assert(not collectgarbage("isrunning"))
  collectgarbage()
  assert(not collectgarbage("isrunning"))
  local a = {}
  for i=1,100 do a[i] = {{}}; local b = {} end
  local x = gcinfo()
  local i = 0
  repeat   -- do steps until it completes a collection cycle
    i = i+1
  until collectgarbage("step", siz)
  assert(gcinfo() < x)
  return i
end

collectgarbage"stop"

if not _port then
  -- test the "size" of basic GC steps (whatever they mean...)
  assert(dosteps(0) > 10)
  assert(dosteps(10) < dosteps(2))
end

-- collector should do a full collection with so many steps
assert(dosteps(100000) == 1)
assert(collectgarbage("step", 1000000) == true)
assert(collectgarbage("step", 1000000) == true)

assert(not collectgarbage("isrunning"))
collectgarbage"restart"
assert(collectgarbage("isrunning"))


if not _port then
  -- test the pace of the collector
  collectgarbage(); collectgarbage()
  local x = gcinfo()
  collectgarbage"stop"
  assert(not collectgarbage("isrunning"))
  repeat
    local a = {}
  until gcinfo() > 3 * x
  collectgarbage"restart"
  assert(collectgarbage("isrunning"))
  repeat
    local a = {}
  until gcinfo() <= x * 2
end


print("clearing tables")
lim = 15
a = {}
-- fill a with `collectable' indices
for i=1,lim do a[{}] = i end
b = {}
for k,v in pairs(a) do b[k]=v end
-- remove all indices and collect them
for n in pairs(b) do
  a[n] = nil
  assert(type(n) == 'table' and next(n) == nil)
  collectgarbage()
end
b = nil
collectgarbage()
for n in pairs(a) do error'cannot be here' end
for i=1,lim do a[i] = i end
for i=1,lim do assert(a[i] == i) end


print('weak tables')
a = {}; setmetatable(a, {__mode = 'k'});
-- fill a with some `collectable' indices
for i=1,lim do a[{}] = i end
-- and some non-collectable ones
for i=1,lim do a[i] = i end
for i=1,lim do local s=string.rep('@', i); a[s] = s..'#' end
collectgarbage()
local i = 0
for k,v in pairs(a) do assert(k==v or k..'#'==v); i=i+1 end
assert(i == 2*lim)

a = {}; setmetatable(a, {__mode = 'v'});
a[1] = string.rep('b', 21)
collectgarbage()
assert(a[1])   -- strings are *values*
a[1] = nil
-- fill a with some `collectable' values (in both parts of the table)
for i=1,lim do a[i] = {} end
for i=1,lim do a[i..'x'] = {} end
-- and some non-collectable ones
for i=1,lim do local t={}; a[t]=t end
for i=1,lim do a[i+lim]=i..'x' end
collectgarbage()
local i = 0
for k,v in pairs(a) do assert(k==v or k-lim..'x' == v); i=i+1 end
assert(i == 2*lim)

a = {}; setmetatable(a, {__mode = 'vk'});
local x, y, z = {}, {}, {}
-- keep only some items
a[1], a[2], a[3] = x, y, z
a[string.rep('$', 11)] = string.rep('$', 11)
-- fill a with some `collectable' values
for i=4,lim do a[i] = {} end
for i=1,lim do a[{}] = i end
for i=1,lim do local t={}; a[t]=t end
collectgarbage()
assert(next(a) ~= nil)
local i = 0
for k,v in pairs(a) do
  assert((k == 1 and v == x) or
         (k == 2 and v == y) or
         (k == 3 and v == z) or k==v);
  i = i+1
end
assert(i == 4)
x,y,z=nil
collectgarbage()
assert(next(a) == string.rep('$', 11))


-- 'bug' in 5.1
a = {}
local t = {x = 10}
local C = setmetatable({key = t}, {__mode = 'v'})
local C1 = setmetatable({[t] = 1}, {__mode = 'k'})
a.x = t  -- this should not prevent 't' from being removed from
         -- weak table 'C' by the time 'a' is finalized

setmetatable(a, {__gc = function (u)
                          assert(C.key == nil)
                          assert(type(next(C1)) == 'table')
                          end})

a, t = nil
collectgarbage()
collectgarbage()
assert(next(C) == nil and next(C1) == nil)
C, C1 = nil


-- ephemerons
local mt = {__mode = 'k'}
a = {10,20,30,40}; setmetatable(a, mt)
x = nil
for i = 1, 100 do local n = {}; a[n] = {k = {x}}; x = n end
GC()
local n = x
local i = 0
while n do n = a[n].k[1]; i = i + 1 end
assert(i == 100)
x = nil
GC()
for i = 1, 4 do assert(a[i] == i * 10); a[i] = nil end
assert(next(a) == nil)

local K = {}
a[K] = {}
for i=1,10 do a[K][i] = {}; a[a[K][i]] = setmetatable({}, mt) end
x = nil
local k = 1
for j = 1,100 do
  local n = {}; local nk = k%10 + 1
  a[a[K][nk]][n] = {x, k = k}; x = n; k = nk
end
GC()
local n = x
local i = 0
while n do local t = a[a[K][k]][n]; n = t[1]; k = t.k; i = i + 1 end
assert(i == 100)
K = nil
GC()
assert(next(a) == nil)


-- testing errors during GC
do
collectgarbage("stop")   -- stop collection
local u = {}
local s = {}; setmetatable(s, {__mode = 'k'})
setmetatable(u, {__gc = function (o)
  local i = s[o]
  s[i] = true
  assert(not s[i - 1])   -- check proper finalization order
  if i == 8 then error("here") end   -- error during GC
end})

for i = 6, 10 do
  local n = setmetatable({}, getmetatable(u))
  s[n] = i
end

assert(not pcall(collectgarbage))
for i = 8, 10 do assert(s[i]) end

for i = 1, 5 do
  local n = setmetatable({}, getmetatable(u))
  s[n] = i
end

collectgarbage()
for i = 1, 10 do assert(s[i]) end

getmetatable(u).__gc = false


-- __gc errors with non-string messages
setmetatable({}, {__gc = function () error{} end})
local a, b = pcall(collectgarbage)
assert(not a and type(b) == "string" and string.find(b, "error in __gc"))

end
print '+'


-- testing userdata
if T==nil then
  (Message or print)('\a\n >>> testC not active: skipping userdata GC tests <<<\n\a')

else

  local function newproxy(u)
    return debug.setmetatable(T.newuserdata(0), debug.getmetatable(u))
  end

  collectgarbage("stop")   -- stop collection
  local u = newproxy(nil)
  debug.setmetatable(u, {__gc = true})
  local s = 0
  local a = {[u] = 0}; setmetatable(a, {__mode = 'vk'})
  for i=1,10 do a[newproxy(u)] = i end
  for k in pairs(a) do assert(getmetatable(k) == getmetatable(u)) end
  local a1 = {}; for k,v in pairs(a) do a1[k] = v end
  for k,v in pairs(a1) do a[v] = k end
  for i =1,10 do assert(a[i]) end
  getmetatable(u).a = a1
  getmetatable(u).u = u
  do
    local u = u
    getmetatable(u).__gc = function (o)
      assert(a[o] == 10-s)
      assert(a[10-s] == nil) -- udata already removed from weak table
      assert(getmetatable(o) == getmetatable(u))
    assert(getmetatable(o).a[o] == 10-s)
      s=s+1
    end
  end
  a1, u = nil
  assert(next(a) ~= nil)
  collectgarbage()
  assert(s==11)
  collectgarbage()
  assert(next(a) == nil)  -- finalized keys are removed in two cycles
end


-- __gc x weak tables
local u = setmetatable({}, {__gc = true})
-- __gc metamethod should be collected before running
setmetatable(getmetatable(u), {__mode = "v"})
getmetatable(u).__gc = function (o) os.exit(1) end  -- cannot happen
u = nil
collectgarbage()

local u = setmetatable({}, {__gc = true})
local m = getmetatable(u)
m.x = {[{0}] = 1; [0] = {1}}; setmetatable(m.x, {__mode = "kv"});
m.__gc = function (o)
  assert(next(getmetatable(o).x) == nil)
  m = 10
end
u, m = nil
collectgarbage()
assert(m==10)


-- errors during collection
u = setmetatable({}, {__gc = function () error "!!!" end})
u = nil
assert(not pcall(collectgarbage))


if not _soft then
  print("deep structures")
  local a = {}
  for i = 1,200000 do
    a = {next = a}
  end
  collectgarbage()
end

-- create many threads with self-references and open upvalues
local thread_id = 0
local threads = {}

local function fn (thread)
    local x = {}
    threads[thread_id] = function()
                             thread = x
                         end
    coroutine.yield()
end

while thread_id < 1000 do
    local thread = coroutine.create(fn)
    coroutine.resume(thread, thread)
    thread_id = thread_id + 1
end

do
  collectgarbage()
  collectgarbage"stop"
  local x = gcinfo()
  repeat
    for i=1,1000 do _ENV.a = {} end
    collectgarbage("step", 1)   -- steps should not unblock the collector
  until gcinfo() > 2 * x
  collectgarbage"restart"
end


if T then   -- tests for weird cases collecting upvalues
  local a = 1200
  local f = function () return a end    -- create upvalue for 'a'
  assert(f() == 1200)

  -- erase reference to upvalue 'a', mark it as dead, but does not collect it
  T.gcstate("pause"); collectgarbage("stop")
  f = nil
  T.gcstate("sweepstring")

  -- this function will reuse that dead upvalue...
  f = function () return a end
  assert(f() == 1200)

  -- create coroutine with local variable 'b'
  local co = coroutine.wrap(function()
    local b = 150
    coroutine.yield(function () return b end)
  end)

  T.gcstate("pause")
  assert(co()() == 150)  -- create upvalue for 'b'

  -- mark upvalue 'b' as dead, but does not collect it
  T.gcstate("sweepstring")

  co()   -- finish coroutine, "closing" that dead upvalue

  assert(f() == 1200)
  collectgarbage("restart")

  print"+"
end


if T then
  local debug = require "debug"
  collectgarbage("generational"); collectgarbage("stop")
  x = T.newuserdata(0)
  T.gcstate("propagate")    -- ensure 'x' is old
  T.gcstate("sweepstring")
  T.gcstate("propagate")
  assert(string.find(T.gccolor(x), "/old"))
  local y = T.newuserdata(0)
  debug.setmetatable(y, {__gc = true})   -- bless the new udata before...
  debug.setmetatable(x, {__gc = true})   -- ...the old one
  assert(string.find(T.gccolor(y), "white"))
  T.checkmemory()
  collectgarbage("incremental"); collectgarbage("restart")
end


if T then
  print("emergency collections")
  collectgarbage()
  collectgarbage()
  T.totalmem(T.totalmem() + 200)
  for i=1,200 do local a = {} end
  T.totalmem(1000000000)
  collectgarbage()
  local t = T.totalmem("table")
  local a = {{}, {}, {}}   -- create 4 new tables
  assert(T.totalmem("table") == t + 4)
  t = T.totalmem("function")
  a = function () end   -- create 1 new closure
  assert(T.totalmem("function") == t + 1)
  t = T.totalmem("thread")
  a = coroutine.create(function () end)   -- create 1 new coroutine
  assert(T.totalmem("thread") == t + 1)
end

-- create an object to be collected when state is closed
do
  local setmetatable,assert,type,print,getmetatable =
        setmetatable,assert,type,print,getmetatable
  local tt = {}
  tt.__gc = function (o)
    assert(getmetatable(o) == tt)
    -- create new objects during GC
    local a = 'xuxu'..(10+3)..'joao', {}
    ___Glob = o  -- ressurect object!
    setmetatable({}, tt)  -- creates a new one with same metatable
    print(">>> closing state " .. "<<<\n")
  end
  local u = setmetatable({}, tt)
  ___Glob = {u}   -- avoid object being collected before program end
end

-- create several objects to raise errors when collected while closing state
do
  local mt = {__gc = function (o) return o + 1 end}
  for i = 1,10 do
    -- create object and preserve it until the end
    table.insert(___Glob, setmetatable({}, mt))
  end
end

-- just to make sure
assert(collectgarbage'isrunning')

print('OK')
