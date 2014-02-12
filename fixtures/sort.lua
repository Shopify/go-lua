print "testing (parts of) table library"

print "testing unpack"

local unpack = table.unpack

local x,y,z,a,n
a = {}; lim = 2000
for i=1, lim do a[i]=i end
assert(select(lim, unpack(a)) == lim and select('#', unpack(a)) == lim)
x = unpack(a)
assert(x == 1)
x = {unpack(a)}
assert(#x == lim and x[1] == 1 and x[lim] == lim)
x = {unpack(a, lim-2)}
assert(#x == 3 and x[1] == lim-2 and x[3] == lim)
x = {unpack(a, 10, 6)}
assert(next(x) == nil)   -- no elements
x = {unpack(a, 11, 10)}
assert(next(x) == nil)   -- no elements
x,y = unpack(a, 10, 10)
assert(x == 10 and y == nil)
x,y,z = unpack(a, 10, 11)
assert(x == 10 and y == 11 and z == nil)
a,x = unpack{1}
assert(a==1 and x==nil)
a,x = unpack({1,2}, 1, 1)
assert(a==1 and x==nil)

if not _no32 then
  assert(not pcall(unpack, {}, 0, 2^31-1))
  assert(not pcall(unpack, {}, 1, 2^31-1))
  assert(not pcall(unpack, {}, -(2^31), 2^31-1))
  assert(not pcall(unpack, {}, -(2^31 - 1), 2^31-1))
  assert(pcall(unpack, {}, 2^31-1, 0))
  assert(pcall(unpack, {}, 2^31-1, 1))
  pcall(unpack, {}, 1, 2^31)
  a, b = unpack({[2^31-1] = 20}, 2^31-1, 2^31-1)
  assert(a == 20 and b == nil)
  a, b = unpack({[2^31-1] = 20}, 2^31-2, 2^31-1)
  assert(a == nil and b == 20)
end

print "testing pack"

a = table.pack()
assert(a[1] == nil and a.n == 0) 

a = table.pack(table)
assert(a[1] == table and a.n == 1)

a = table.pack(nil, nil, nil, nil)
assert(a[1] == nil and a.n == 4)


print"testing sort"


-- test checks for invalid order functions
local function check (t)
  local function f(a, b) assert(a and b); return true end
  local s, e = pcall(table.sort, t, f)
  assert(not s and e:find("invalid order function"))
end

check{1,2,3,4}
check{1,2,3,4,5}
check{1,2,3,4,5,6}


function check (a, f)
  f = f or function (x,y) return x<y end;
  for n = #a, 2, -1 do
    assert(not f(a[n], a[n-1]))
  end
end

a = {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep",
     "Oct", "Nov", "Dec"}

table.sort(a)
check(a)

function perm (s, n)
  n = n or #s
  if n == 1 then
    local t = {unpack(s)}
    table.sort(t)
    check(t)
  else
    for i = 1, n do
      s[i], s[n] = s[n], s[i]
      perm(s, n - 1)
      s[i], s[n] = s[n], s[i]
    end
  end
end

perm{}
perm{1}
perm{1,2}
perm{1,2,3}
perm{1,2,3,4}
perm{2,2,3,4}
perm{1,2,3,4,5}
perm{1,2,3,3,5}
perm{1,2,3,4,5,6}
perm{2,2,3,3,5,6}

limit = 30000
if _soft then limit = 5000 end

a = {}
for i=1,limit do
  a[i] = math.random()
end

local x = os.clock()
table.sort(a)
print(string.format("Sorting %d elements in %.2f sec.", limit, os.clock()-x))
check(a)

x = os.clock()
table.sort(a)
print(string.format("Re-sorting %d elements in %.2f sec.", limit, os.clock()-x))
check(a)

a = {}
for i=1,limit do
  a[i] = math.random()
end

x = os.clock(); i=0
table.sort(a, function(x,y) i=i+1; return y<x end)
print(string.format("Invert-sorting other %d elements in %.2f sec., with %i comparisons",
      limit, os.clock()-x, i))
check(a, function(x,y) return y<x end)


table.sort{}  -- empty array

for i=1,limit do a[i] = false end
x = os.clock();
table.sort(a, function(x,y) return nil end)
print(string.format("Sorting %d equal elements in %.2f sec.", limit, os.clock()-x))
check(a, function(x,y) return nil end)
for i,v in pairs(a) do assert(not v or i=='n' and v==limit) end

A = {"álo", "\0first :-)", "alo", "then this one", "45", "and a new"}
table.sort(A)
check(A)

table.sort(A, function (x, y)
          load(string.format("A[%q] = ''", x))()
          collectgarbage()
          return x<y
        end)


tt = {__lt = function (a,b) return a.val < b.val end}
a = {}
for i=1,10 do  a[i] = {val=math.random(100)}; setmetatable(a[i], tt); end
table.sort(a)
check(a, tt.__lt)
check(a)

print"OK"
