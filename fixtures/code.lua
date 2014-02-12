if T==nil then
  (Message or print)('\a\n >>> testC not active: skipping opcode tests <<<\n\a')
  return
end
print "testing code generation and optimizations"


-- this code gave an error for the code checker
do
  local function f (a)
  for k,v,w in a do end
  end
end


function check (f, ...)
  local arg = {...}
  local c = T.listcode(f)
  for i=1, #arg do
    -- print(arg[i], c[i])
    assert(string.find(c[i], '- '..arg[i]..' *%d'))
  end
  assert(c[#arg+2] == nil)
end


function checkequal (a, b)
  a = T.listcode(a)
  b = T.listcode(b)
  for i = 1, #a do
    a[i] = string.gsub(a[i], '%b()', '')   -- remove line number
    b[i] = string.gsub(b[i], '%b()', '')   -- remove line number
    assert(a[i] == b[i])
  end
end


-- some basic instructions
check(function ()
  (function () end){f()}
end, 'CLOSURE', 'NEWTABLE', 'GETTABUP', 'CALL', 'SETLIST', 'CALL', 'RETURN')


-- sequence of LOADNILs
check(function ()
  local a,b,c
  local d; local e;
  local f,g,h;
  d = nil; d=nil; b=nil; a=nil; c=nil;
end, 'LOADNIL', 'RETURN')

check(function ()
  local a,b,c,d = 1,1,1,1
  d=nil;c=nil;b=nil;a=nil
end, 'LOADK', 'LOADK', 'LOADK', 'LOADK', 'LOADNIL', 'RETURN')

do
  local a,b,c,d = 1,1,1,1
  d=nil;c=nil;b=nil;a=nil
  assert(a == nil and b == nil and c == nil and d == nil)
end


-- single return
check (function (a,b,c) return a end, 'RETURN')


-- infinite loops
check(function () while true do local a = -1 end end,
'LOADK', 'JMP', 'RETURN')

check(function () while 1 do local a = -1 end end,
'LOADK', 'JMP', 'RETURN')

check(function () repeat local x = 1 until true end,
'LOADK', 'RETURN')


-- concat optimization
check(function (a,b,c,d) return a..b..c..d end,
  'MOVE', 'MOVE', 'MOVE', 'MOVE', 'CONCAT', 'RETURN')

-- not
check(function () return not not nil end, 'LOADBOOL', 'RETURN')
check(function () return not not false end, 'LOADBOOL', 'RETURN')
check(function () return not not true end, 'LOADBOOL', 'RETURN')
check(function () return not not 1 end, 'LOADBOOL', 'RETURN')

-- direct access to locals
check(function ()
  local a,b,c,d
  a = b*2
  c[4], a[b] = -((a + d/-20.5 - a[b]) ^ a.x), b
end,
  'LOADNIL',
  'MUL',
  'DIV', 'ADD', 'GETTABLE', 'SUB', 'GETTABLE', 'POW',
    'UNM', 'SETTABLE', 'SETTABLE', 'RETURN')


-- direct access to constants
check(function ()
  local a,b
  a.x = 0
  a.x = b
  a[b] = 'y'
  a = 1 - a
  b = 1/a
  b = 5+4
  a[true] = false
end,
  'LOADNIL',
  'SETTABLE', 'SETTABLE', 'SETTABLE', 'SUB', 'DIV', 'LOADK',
  'SETTABLE', 'RETURN')

-- constant folding
local function f () return -((2^8 + -(-1)) % 8)/2 * 4 - 3 end

check(f, 'LOADK', 'RETURN')
assert(f() == -5)


-- bug in constant folding for 5.1
check(function () return -nil end,
  'LOADNIL', 'UNM', 'RETURN')


check(function ()
  local a,b,c
  b[c], a = c, b
  b[a], a = c, b
  a, b = c, a
  a = a
end, 
  'LOADNIL',
  'MOVE', 'MOVE', 'SETTABLE',
  'MOVE', 'MOVE', 'MOVE', 'SETTABLE',
  'MOVE', 'MOVE', 'MOVE',
  -- no code for a = a
  'RETURN')


-- x == nil , x ~= nil
checkequal(function () if (a==nil) then a=1 end; if a~=nil then a=1 end end,
           function () if (a==9) then a=1 end; if a~=9 then a=1 end end)

check(function () if a==nil then a=1 end end,
'GETTABUP', 'EQ', 'JMP', 'SETTABUP', 'RETURN')

-- de morgan
checkequal(function () local a; if not (a or b) then b=a end end,
           function () local a; if (not a and not b) then b=a end end)

checkequal(function (l) local a; return 0 <= a and a <= l end,
           function (l) local a; return not (not(a >= 0) or not(a <= l)) end)


-- if-goto optimizations
check(function (a)
        if a == 1 then goto l1
        elseif a == 2 then goto l2
        elseif a == 3 then goto l2
        else if a == 4 then goto l3
             else goto l3
             end
        end
        ::l1:: ::l2:: ::l3:: ::l4:: 
end, 'EQ', 'JMP', 'EQ', 'JMP', 'EQ', 'JMP', 'EQ', 'JMP', 'JMP', 'RETURN')

checkequal(
function (a) while a < 10 do a = a + 1 end end,
function (a) ::L2:: if not(a < 10) then goto L1 end; a = a + 1;
                goto L2; ::L1:: end
)

checkequal(
function (a) while a < 10 do a = a + 1 end end,
function (a) while true do if not(a < 10) then break end; a = a + 1; end end
)

print 'OK'

