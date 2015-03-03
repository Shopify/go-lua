require('math')
require('table')

local N = 100000

local a = {}
for i=1,N do
  a[i] = math.random()
end

local i = 0
table.sort(a, function(x,y) i=i+1; return y<x end)

print(i)
