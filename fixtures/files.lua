debug = require "debug"

assert(type(os.getenv"PATH") == "string")

assert(io.input(io.stdin) == io.stdin)
assert(not pcall(io.input, "non-existent-file"))
assert(io.output(io.stdout) == io.stdout)

-- cannot close standard files
assert(not io.close(io.stdin) and
       not io.stdout:close() and
       not io.stderr:close())


assert(type(io.input()) == "userdata" and io.type(io.output()) == "file")
assert(type(io.stdin) == "userdata" and io.type(io.stderr) == "file")
assert(io.type(8) == nil)
local a = {}; setmetatable(a, {})
assert(io.type(a) == nil)

local a,b,c = io.open('xuxu_nao_existe')
assert(not a and type(b) == "string" and type(c) == "number")

a,b,c = io.open('/a/b/c/d', 'w')
assert(not a and type(b) == "string" and type(c) == "number")

local file = os.tmpname()
local f, msg = io.open(file, "w")
if not f then
  (Message or print)("'os.tmpname' file cannot be open; skipping file tests")

else  --{  most tests here need tmpname
f:close()

print('testing i/o')

local otherfile = os.tmpname()

assert(not pcall(io.open, file, "rw"))     -- invalid mode
assert(not pcall(io.open, file, "rb+"))    -- invalid mode
assert(not pcall(io.open, file, "r+bk"))   -- invalid mode
assert(not pcall(io.open, file, ""))       -- invalid mode
assert(not pcall(io.open, file, "+"))      -- invalid mode
assert(not pcall(io.open, file, "b"))      -- invalid mode
assert(io.open(file, "r+b")):close()
assert(io.open(file, "r+")):close()
assert(io.open(file, "rb")):close()

assert(os.setlocale('C', 'all'))

io.input(io.stdin); io.output(io.stdout);

os.remove(file)
assert(loadfile(file) == nil)
assert(io.open(file) == nil)
io.output(file)
assert(io.output() ~= io.stdout)

assert(io.output():seek() == 0)
assert(io.write("alo alo"):seek() == string.len("alo alo"))
assert(io.output():seek("cur", -3) == string.len("alo alo")-3)
assert(io.write("joao"))
assert(io.output():seek("end") == string.len("alo joao"))

assert(io.output():seek("set") == 0)

assert(io.write('"álo"', "{a}\n", "second line\n", "third line \n"))
assert(io.write('çfourth_line'))
io.output(io.stdout)
collectgarbage()  -- file should be closed by GC
assert(io.input() == io.stdin and rawequal(io.output(), io.stdout))
print('+')

-- test GC for files
collectgarbage()
for i=1,120 do
  for i=1,5 do
    io.input(file)
    assert(io.open(file, 'r'))
    io.lines(file)
  end
  collectgarbage()
end

io.input():close()
io.close()

assert(os.rename(file, otherfile))
assert(os.rename(file, otherfile) == nil)

io.output(io.open(otherfile, "ab"))
assert(io.write("\n\n\t\t  3450\n"));
io.close()

-- test line generators
assert(not pcall(io.lines, "non-existent-file"))
assert(os.rename(otherfile, file))
io.output(otherfile)
local f = io.lines(file)
while f() do end;
assert(not pcall(f))  -- read lines after EOF
assert(not pcall(f))  -- read lines after EOF
-- copy from file to otherfile
for l in io.lines(file) do io.write(l, "\n") end
io.close()
-- copy from otherfile back to file
local f = assert(io.open(otherfile))
assert(io.type(f) == "file")
io.output(file)
assert(io.output():read() == nil)
for l in f:lines() do io.write(l, "\n") end
assert(tostring(f):sub(1, 5) == "file ")
assert(f:close()); io.close()
assert(not pcall(io.close, f))   -- error trying to close again
assert(tostring(f) == "file (closed)")
assert(io.type(f) == "closed file")
io.input(file)
f = io.open(otherfile):lines()
for l in io.lines() do assert(l == f()) end
f = nil; collectgarbage()
assert(os.remove(otherfile))

io.input(file)
do  -- test error returns
  local a,b,c = io.input():write("xuxu")
  assert(not a and type(b) == "string" and type(c) == "number")
end
assert(io.read(0) == "")   -- not eof
assert(io.read(5, '*l') == '"álo"')
assert(io.read(0) == "")
assert(io.read() == "second line")
local x = io.input():seek()
assert(io.read() == "third line ")
assert(io.input():seek("set", x))
assert(io.read('*L') == "third line \n")
assert(io.read(1) == "ç")
assert(io.read(string.len"fourth_line") == "fourth_line")
assert(io.input():seek("cur", -string.len"fourth_line"))
assert(io.read() == "fourth_line")
assert(io.read() == "")  -- empty line
assert(io.read('*n') == 3450)
assert(io.read(1) == '\n')
assert(io.read(0) == nil)  -- end of file
assert(io.read(1) == nil)  -- end of file
assert(io.read(30000) == nil)  -- end of file
assert(({io.read(1)})[2] == nil)
assert(io.read() == nil)  -- end of file
assert(({io.read()})[2] == nil)
assert(io.read('*n') == nil)  -- end of file
assert(({io.read('*n')})[2] == nil)
assert(io.read('*a') == '')  -- end of file (OK for `*a')
assert(io.read('*a') == '')  -- end of file (OK for `*a')
collectgarbage()
print('+')
io.close(io.input())
assert(not pcall(io.read))

assert(os.remove(file))

local t = '0123456789'
for i=1,12 do t = t..t; end
assert(string.len(t) == 10*2^12)

io.output(file)
io.write("alo"):write("\n")
io.close()
assert(not pcall(io.write))
local f = io.open(file, "a+b")
io.output(f)
collectgarbage()

assert(io.write(' ' .. t .. ' '))
assert(io.write(';', 'end of file\n'))
f:flush(); io.flush()
f:close()
print('+')

io.input(file)
assert(io.read() == "alo")
assert(io.read(1) == ' ')
assert(io.read(string.len(t)) == t)
assert(io.read(1) == ' ')
assert(io.read(0))
assert(io.read('*a') == ';end of file\n')
assert(io.read(0) == nil)
assert(io.close(io.input()))


-- test errors in read/write
do
  local function ismsg (m)
    -- error message is not a code number
    return (type(m) == "string" and tonumber(m) == nil)
  end

  -- read
  local f = io.open(file, "w")
  local r, m, c = f:read()
  assert(r == nil and ismsg(m) and type(c) == "number")
  assert(f:close())
  -- write
  f = io.open(file, "r")
  r, m, c = f:write("whatever")
  assert(r == nil and ismsg(m) and type(c) == "number")
  assert(f:close())
  -- lines
  f = io.open(file, "w")
  r, m = pcall(f:lines())
  assert(r == false and ismsg(m))
  assert(f:close())
end

assert(os.remove(file))

-- test for *L format
io.output(file); io.write"\n\nline\nother":close()
io.input(file)
assert(io.read"*L" == "\n")
assert(io.read"*L" == "\n")
assert(io.read"*L" == "line\n")
assert(io.read"*L" == "other")
assert(io.read"*L" == nil)
io.input():close()

local f = assert(io.open(file))
local s = ""
for l in f:lines("*L") do s = s .. l end
assert(s == "\n\nline\nother")
f:close()

io.input(file)
s = ""
for l in io.lines(nil, "*L") do s = s .. l end
assert(s == "\n\nline\nother")
io.input():close()

s = ""
for l in io.lines(file, "*L") do s = s .. l end
assert(s == "\n\nline\nother")

s = ""
for l in io.lines(file, "*l") do s = s .. l end
assert(s == "lineother")

io.output(file); io.write"a = 10 + 34\na = 2*a\na = -a\n":close()
local t = {}
load(io.lines(file, "*L"), nil, nil, t)()
assert(t.a == -((10 + 34) * 2))


-- test for multipe arguments in 'lines'
io.output(file); io.write"0123456789\n":close()
for a,b in io.lines(file, 1, 1) do
  if a == "\n" then assert(b == nil)
  else assert(tonumber(a) == b - 1)
  end
end

for a,b,c in io.lines(file, 1, 2, "*a") do
  assert(a == "0" and b == "12" and c == "3456789\n")
end

for a,b,c in io.lines(file, "*a", 0, 1) do
  if a == "" then break end
  assert(a == "0123456789\n" and b == nil and c == nil)
end
collectgarbage()   -- to close file in previous iteration

io.output(file); io.write"00\n10\n20\n30\n40\n":close()
for a, b in io.lines(file, "*n", "*n") do
  if a == 40 then assert(b == nil)
  else assert(a == b - 10)
  end
end


-- test load x lines
io.output(file);
io.write[[
local y
= X
X =
X *
2 +
X;
X =
X
-                                   y;
]]:close()
_G.X = 1
assert(not load(io.lines(file)))
collectgarbage()   -- to close file in previous iteration
load(io.lines(file, "*L"))()
assert(_G.X == 2)
load(io.lines(file, 1))()
assert(_G.X == 4)
load(io.lines(file, 3))()
assert(_G.X == 8)

print('+')

local x1 = "string\n\n\\com \"\"''coisas [[estranhas]] ]]'"
io.output(file)
assert(io.write(string.format("x2 = %q\n-- comment without ending EOS", x1)))
io.close()
assert(loadfile(file))()
assert(x1 == x2)
print('+')
assert(os.remove(file))
assert(os.remove(file) == nil)
assert(os.remove(otherfile) == nil)

-- testing loadfile
local function testloadfile (s, expres)
  io.output(file)
  if s then io.write(s) end
  io.close()
  local res = assert(loadfile(file))()
  assert(os.remove(file))
  assert(res == expres)
end

-- loading empty file
testloadfile(nil, nil)

-- loading file with initial comment without end of line
testloadfile("# a non-ending comment", nil)


-- checking Unicode BOM in files
testloadfile("\xEF\xBB\xBF# some comment\nreturn 234", 234)
testloadfile("\xEF\xBB\xBFreturn 239", 239)
testloadfile("\xEF\xBB\xBF", nil)   -- empty file with a BOM


-- checking line numbers in files with initial comments
testloadfile("# a comment\nreturn debug.getinfo(1).currentline", 2)


-- loading binary file
io.output(io.open(file, "wb"))
assert(io.write(string.dump(function () return 10, '\0alo\255', 'hi' end)))
io.close()
a, b, c = assert(loadfile(file))()
assert(a == 10 and b == "\0alo\255" and c == "hi")
assert(os.remove(file))

-- bug in 5.2.1
do
  io.output(io.open(file, "wb"))
  -- save function with no upvalues
  assert(io.write(string.dump(function () return 1 end)))
  io.close()
  f = assert(loadfile(file, "b", {}))
  assert(type(f) == "function" and f() == 1)
  assert(os.remove(file))
end

-- loading binary file with initial comment
io.output(io.open(file, "wb"))
assert(io.write("#this is a comment for a binary file\0\n",
                string.dump(function () return 20, '\0\0\0' end)))
io.close()
a, b, c = assert(loadfile(file))()
assert(a == 20 and b == "\0\0\0" and c == nil)
assert(os.remove(file))


-- 'loadfile' with 'env'
do
  local f = io.open(file, 'w')
  f:write[[
    if (...) then a = 15; return b, c, d
    else return _ENV
    end
  ]]
  f:close()
  local t = {b = 12, c = "xuxu", d = print}
  local f = assert(loadfile(file, 't', t))
  local b, c, d = f(1)
  assert(t.a == 15 and b == 12 and c == t.c and d == print)
  assert(f() == t)
  f = assert(loadfile(file, 't', nil))
  assert(f() == nil)
  f = assert(loadfile(file))
  assert(f() == _G)
  assert(os.remove(file))
end


-- 'loadfile' x modes
do
  io.open(file, 'w'):write("return 10"):close()
  local s, m = loadfile(file, 'b')
  assert(not s and string.find(m, "a text chunk"))
  io.open(file, 'w'):write("\27 return 10"):close()
  local s, m = loadfile(file, 't')
  assert(not s and string.find(m, "a binary chunk"))
  assert(os.remove(file))
end


io.output(file)
assert(io.write("qualquer coisa\n"))
assert(io.write("mais qualquer coisa"))
io.close()
assert(io.output(assert(io.open(otherfile, 'wb')))
       :write("outra coisa\0\1\3\0\0\0\0\255\0")
       :close())

local filehandle = assert(io.open(file, 'r+'))
local otherfilehandle = assert(io.open(otherfile, 'rb'))
assert(filehandle ~= otherfilehandle)
assert(type(filehandle) == "userdata")
assert(filehandle:read('*l') == "qualquer coisa")
io.input(otherfilehandle)
assert(io.read(string.len"outra coisa") == "outra coisa")
assert(filehandle:read('*l') == "mais qualquer coisa")
filehandle:close();
assert(type(filehandle) == "userdata")
io.input(otherfilehandle)
assert(io.read(4) == "\0\1\3\0")
assert(io.read(3) == "\0\0\0")
assert(io.read(0) == "")        -- 255 is not eof
assert(io.read(1) == "\255")
assert(io.read('*a') == "\0")
assert(not io.read(0))
assert(otherfilehandle == io.input())
otherfilehandle:close()
assert(os.remove(file))
assert(os.remove(otherfile))
collectgarbage()

io.output(file)
  :write[[
 123.4	-56e-2  not a number
second line
third line

and the rest of the file
]]
  :close()
io.input(file)
local _,a,b,c,d,e,h,__ = io.read(1, '*n', '*n', '*l', '*l', '*l', '*a', 10)
assert(io.close(io.input()))
assert(_ == ' ' and __ == nil)
assert(type(a) == 'number' and a==123.4 and b==-56e-2)
assert(d=='second line' and e=='third line')
assert(h==[[

and the rest of the file
]])
assert(os.remove(file))
collectgarbage()

-- testing buffers
do
  local f = assert(io.open(file, "w"))
  local fr = assert(io.open(file, "r"))
  assert(f:setvbuf("full", 2000))
  f:write("x")
  assert(fr:read("*all") == "")  -- full buffer; output not written yet
  f:close()
  fr:seek("set")
  assert(fr:read("*all") == "x")   -- `close' flushes it
  f = assert(io.open(file), "w")
  assert(f:setvbuf("no"))
  f:write("x")
  fr:seek("set")
  assert(fr:read("*all") == "x")  -- no buffer; output is ready
  f:close()
  f = assert(io.open(file, "a"))
  assert(f:setvbuf("line"))
  f:write("x")
  fr:seek("set", 1)
  assert(fr:read("*all") == "")   -- line buffer; no output without `\n'
  f:write("a\n"):seek("set", 1)
  assert(fr:read("*all") == "xa\n")  -- now we have a whole line
  f:close(); fr:close()
  assert(os.remove(file))
end


if not _soft then
  print("testing large files (> BUFSIZ)")
  io.output(file)
  for i=1,5001 do io.write('0123456789123') end
  io.write('\n12346'):close()
  io.input(file)
  local x = io.read('*a')
  io.input():seek('set', 0)
  local y = io.read(30001)..io.read(1005)..io.read(0)..
            io.read(1)..io.read(100003)
  assert(x == y and string.len(x) == 5001*13 + 6)
  io.input():seek('set', 0)
  y = io.read()  -- huge line
  assert(x == y..'\n'..io.read())
  assert(io.read() == nil)
  io.close(io.input())
  assert(os.remove(file))
  x = nil; y = nil
end

if not _noposix then
  print("testing popen/pclose and execute")
  local tests = {
    -- command,   what,  code
    {"ls > /dev/null", "ok"},
    {"not-to-be-found-command", "exit"},
    {"exit 3", "exit", 3},
    {"exit 129", "exit", 129},
    {"kill -s HUP $$", "signal", 1},
    {"kill -s KILL $$", "signal", 9},
    {"sh -c 'kill -s HUP $$'", "exit"},
    {'lua -e "os.exit(20, true)"', "exit", 20},
  }
  print("\n(some error messages are expected now)")
  for _, v in ipairs(tests) do
    local x, y, z = io.popen(v[1]):close()
    local x1, y1, z1 = os.execute(v[1])
    assert(x == x1 and y == y1 and z == z1)
    if v[2] == "ok" then
      assert(x == true and y == 'exit' and z == 0)
    else
      assert(x == nil and y == v[2])   -- correct status and 'what'
      -- correct code if known (but always different from 0)
      assert((v[3] == nil and z > 0) or v[3] == z)
    end
  end
end


-- testing tmpfile
f = io.tmpfile()
assert(io.type(f) == "file")
f:write("alo")
f:seek("set")
assert(f:read"*a" == "alo")

end --}

print'+'


assert(os.date("") == "")
assert(os.date("!") == "")
local x = string.rep("a", 10000)
assert(os.date(x) == x)
local t = os.time()
D = os.date("*t", t)
assert(os.date(string.rep("%d", 1000), t) ==
       string.rep(os.date("%d", t), 1000))
assert(os.date(string.rep("%", 200)) == string.rep("%", 100))

local t = os.time()
D = os.date("*t", t)
load(os.date([[assert(D.year==%Y and D.month==%m and D.day==%d and
  D.hour==%H and D.min==%M and D.sec==%S and
  D.wday==%w+1 and D.yday==%j and type(D.isdst) == 'boolean')]], t))()

assert(not pcall(os.date, "%9"))   -- invalid conversion specifier
assert(not pcall(os.date, "%"))   -- invalid conversion specifier
assert(not pcall(os.date, "%O"))   -- invalid conversion specifier
assert(not pcall(os.date, "%E"))   -- invalid conversion specifier
assert(not pcall(os.date, "%Ea"))   -- invalid conversion specifier

if not _noposix then
  assert(type(os.date("%Ex")) == 'string')
  assert(type(os.date("%Oy")) == 'string')
end

assert(os.time(D) == t)
assert(not pcall(os.time, {hour = 12}))

D = os.date("!*t", t)
load(os.date([[!assert(D.year==%Y and D.month==%m and D.day==%d and
  D.hour==%H and D.min==%M and D.sec==%S and
  D.wday==%w+1 and D.yday==%j and type(D.isdst) == 'boolean')]], t))()

do
  local D = os.date("*t")
  local t = os.time(D)
  assert(type(D.isdst) == 'boolean')
  D.isdst = nil
  local t1 = os.time(D)
  assert(t == t1)   -- if isdst is absent uses correct default
end

t = os.time(D)
D.year = D.year-1;
local t1 = os.time(D)
-- allow for leap years
assert(math.abs(os.difftime(t,t1)/(24*3600) - 365) < 2)

t = os.time()
t1 = os.time(os.date("*t"))
assert(os.difftime(t1,t) <= 2)

local t1 = os.time{year=2000, month=10, day=1, hour=23, min=12}
local t2 = os.time{year=2000, month=10, day=1, hour=23, min=10, sec=19}
assert(os.difftime(t1,t2) == 60*2-19)

io.output(io.stdout)
local d = os.date('%d')
local m = os.date('%m')
local a = os.date('%Y')
local ds = os.date('%w') + 1
local h = os.date('%H')
local min = os.date('%M')
local s = os.date('%S')
io.write(string.format('test done on %2.2d/%2.2d/%d', d, m, a))
io.write(string.format(', at %2.2d:%2.2d:%2.2d\n', h, min, s))
io.write(string.format('%s\n', _VERSION))


