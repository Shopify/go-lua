# testing special comment on first line

-- most (all?) tests here assume a reasonable "Unix-like" shell
if _port then return end

print ("testing lua.c options")

assert(os.execute())   -- machine has a system command

prog = os.tmpname()
otherprog = os.tmpname()
out = os.tmpname()

do
  local i = 0
  while arg[i] do i=i-1 end
  progname = arg[i+1]
end
print("progname: "..progname)

local prepfile = function (s, p)
  p = p or prog
  io.output(p)
  io.write(s)
  assert(io.close())
end

function getoutput ()
  io.input(out)
  local t = io.read("*a")
  io.input():close()
  assert(os.remove(out))
  return t
end

function checkprogout (s)
  local t = getoutput()
  for line in string.gmatch(s, ".-\n") do
    assert(string.find(t, line, 1, true))
  end
end

function checkout (s)
  local t = getoutput()
  if s ~= t then print(string.format("'%s' - '%s'\n", s, t)) end
  assert(s == t)
  return t
end

function auxrun (...)
  local s = string.format(...)
  s = string.gsub(s, "lua", '"'..progname..'"', 1)
  return os.execute(s)
end

function RUN (...)
  assert(auxrun(...))
end

function NoRun (...)
  assert(not auxrun(...))
end

function NoRunMsg (...)
  print("\n(the next error is expected by the test)")
  return NoRun(...)
end

-- test environment variables used by Lua
prepfile("print(package.path)")

RUN("env LUA_INIT= LUA_PATH=x lua %s > %s", prog, out)
checkout("x\n")

RUN("env LUA_INIT= LUA_PATH_5_2=y LUA_PATH=x lua %s > %s", prog, out)
checkout("y\n")

prepfile("print(package.cpath)")

RUN("env LUA_INIT= LUA_CPATH=xuxu lua %s > %s", prog, out)
checkout("xuxu\n")

RUN("env LUA_INIT= LUA_CPATH_5_2=yacc LUA_CPATH=x lua %s > %s", prog, out)
checkout("yacc\n")

prepfile("print(X)")
RUN('env LUA_INIT="X=3" lua %s > %s', prog, out)
checkout("3\n")

prepfile("print(X)")
RUN('env LUA_INIT_5_2="X=10" LUA_INIT="X=3" lua %s > %s', prog, out)
checkout("10\n")

-- test option '-E'
prepfile("print(package.path, package.cpath)")
RUN('env LUA_INIT="error(10)" LUA_PATH=xxx LUA_CPATH=xxx lua -E %s > %s',
     prog, out)
local defaultpath = getoutput()
defaultpath = string.match(defaultpath, "^(.-)\t")   -- remove tab
assert(not string.find(defaultpath, "xxx") and string.find(defaultpath, "lua"))


-- test replacement of ';;' to default path
local function convert (p)
  prepfile("print(package.path)")
  RUN('env LUA_PATH="%s" lua %s > %s', p, prog, out)
  local expected = getoutput()
  expected = string.sub(expected, 1, -2)   -- cut final end of line
  assert(string.gsub(p, ";;", ";"..defaultpath..";") == expected)
end

convert(";")
convert(";;")
convert(";;;")
convert(";;;;")
convert(";;;;;")
convert(";;a;;;bc")


-- test 2 files
prepfile("print(1); a=2; return {x=15}")
prepfile(("print(a); print(_G['%s'].x)"):format(prog), otherprog)
RUN('env LUA_PATH="?;;" lua -l %s -l%s -lstring -l io %s > %s', prog, otherprog, otherprog, out)
checkout("1\n2\n15\n2\n15\n")

local a = [[
  assert(#arg == 3 and arg[1] == 'a' and
         arg[2] == 'b' and arg[3] == 'c')
  assert(arg[-1] == '--' and arg[-2] == "-e " and arg[-3] == '%s')
  assert(arg[4] == nil and arg[-4] == nil)
  local a, b, c = ...
  assert(... == 'a' and a == 'a' and b == 'b' and c == 'c')
]]
a = string.format(a, progname)
prepfile(a)
RUN('lua "-e " -- %s a b c', prog)

prepfile"assert(arg==nil)"
prepfile("assert(arg)", otherprog)
RUN('env LUA_PATH="?;;" lua -l%s - < %s', prog, otherprog)

prepfile""
RUN("lua - < %s > %s", prog, out)
checkout("")

-- test many arguments
prepfile[[print(({...})[30])]]
RUN("lua %s %s > %s", prog, string.rep(" a", 30), out)
checkout("a\n")

RUN([[lua "-eprint(1)" -ea=3 -e "print(a)" > %s]], out)
checkout("1\n3\n")

prepfile[[
  print(
1, a
)
]]
RUN("lua - < %s > %s", prog, out)
checkout("1\tnil\n")

prepfile[[
= (6*2-6) -- ===
a 
= 10
print(a)
= a]]
RUN([[lua -e"_PROMPT='' _PROMPT2=''" -i < %s > %s]], prog, out)
checkprogout("6\n10\n10\n\n")

prepfile("a = [[b\nc\nd\ne]]\n=a")
print("temporary program file: "..prog)
RUN([[lua -e"_PROMPT='' _PROMPT2=''" -i < %s > %s]], prog, out)
checkprogout("b\nc\nd\ne\n\n")

prompt = "alo"
prepfile[[ --
a = 2
]]
RUN([[lua "-e_PROMPT='%s'" -i < %s > %s]], prompt, prog, out)
local t = getoutput()
assert(string.find(t, prompt .. ".*" .. prompt .. ".*" .. prompt))

-- test for error objects
prepfile[[
debug = require "debug"
m = {x=0}
setmetatable(m, {__tostring = function(x)
  return debug.getinfo(4).currentline + x.x
end})
error(m)
]]
NoRun([[lua %s 2> %s]], prog, out)   -- no message
checkout(progname..": 6\n")


s = [=[ -- 
function f ( x ) 
  local a = [[
xuxu
]]
  local b = "\
xuxu\n"
  if x == 11 then return 1 , 2 end  --[[ test multiple returns ]]
  return x + 1 
  --\\
end
=( f( 10 ) )
assert( a == b )
=f( 11 )  ]=]
s = string.gsub(s, ' ', '\n\n')
prepfile(s)
RUN([[lua -e"_PROMPT='' _PROMPT2=''" -i < %s > %s]], prog, out)
checkprogout("11\n1\t2\n\n")
  
prepfile[[#comment in 1st line without \n at the end]]
RUN("lua %s", prog)
  
prepfile[[#test line number when file starts with comment line
debug = require"debug"
print(debug.getinfo(1).currentline)
]]
RUN("lua %s > %s", prog, out)
checkprogout('3')

-- close Lua with an open file
prepfile(string.format([[io.output(%q); io.write('alo')]], out))
RUN("lua %s", prog)
checkout('alo')

-- bug in 5.2 beta (extra \0 after version line)
RUN([[lua -v  -e'print"hello"' > %s]], out)
t = getoutput()
assert(string.find(t, "PUC%-Rio\nhello"))


-- testing os.exit
prepfile("os.exit(nil, true)")
RUN("lua %s", prog)
prepfile("os.exit(0, true)")
RUN("lua %s", prog)
prepfile("os.exit(true, true)")
RUN("lua %s", prog)
prepfile("os.exit(1, true)")
NoRun("lua %s", prog)   -- no message
prepfile("os.exit(false, true)")
NoRun("lua %s", prog)   -- no message

assert(os.remove(prog))
assert(os.remove(otherprog))
assert(not os.remove(out))

RUN("lua -v")

NoRunMsg("lua -h")
NoRunMsg("lua -e")
NoRunMsg("lua -e a")
NoRunMsg("lua -f")

print("OK")
