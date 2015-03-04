require('math')
require('table')
require('os')
require('string')

function check (a, f)
  f = f or function (x,y) return x<y end;
  for n = #a, 2, -1 do
    assert(not f(a[n], a[n-1]))
  end
end

limit = 1000000

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
