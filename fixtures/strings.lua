print('testing strings and string library')

assert('alo' < 'alo1')
assert('' < 'a')
assert('alo\0alo' < 'alo\0b')
assert('alo\0alo\0\0' > 'alo\0alo\0')
assert('alo' < 'alo\0')
assert('alo\0' > 'alo')
assert('\0' < '\1')
assert('\0\0' < '\0\1')
assert('\1\0a\0a' <= '\1\0a\0a')
assert(not ('\1\0a\0b' <= '\1\0a\0a'))
assert('\0\0\0' < '\0\0\0\0')
assert(not('\0\0\0\0' < '\0\0\0'))
assert('\0\0\0' <= '\0\0\0\0')
assert(not('\0\0\0\0' <= '\0\0\0'))
assert('\0\0\0' <= '\0\0\0')
assert('\0\0\0' >= '\0\0\0')
assert(not ('\0\0b' < '\0\0a\0'))
print('+')

assert(string.sub("123456789",2,4) == "234")
assert(string.sub("123456789",7) == "789")
assert(string.sub("123456789",7,6) == "")
assert(string.sub("123456789",7,7) == "7")
assert(string.sub("123456789",0,0) == "")
assert(string.sub("123456789",-10,10) == "123456789")
assert(string.sub("123456789",1,9) == "123456789")
assert(string.sub("123456789",-10,-20) == "")
assert(string.sub("123456789",-1) == "9")
assert(string.sub("123456789",-4) == "6789")
assert(string.sub("123456789",-6, -4) == "456")
if not _no32 then
  assert(string.sub("123456789",-2^31, -4) == "123456")
  assert(string.sub("123456789",-2^31, 2^31 - 1) == "123456789")
  assert(string.sub("123456789",-2^31, -2^31) == "")
end
assert(string.sub("\000123456789",3,5) == "234")
assert(("\000123456789"):sub(8) == "789")
print('+')

assert(string.find("123456789", "345") == 3)
a,b = string.find("123456789", "345")
assert(string.sub("123456789", a, b) == "345")
assert(string.find("1234567890123456789", "345", 3) == 3)
assert(string.find("1234567890123456789", "345", 4) == 13)
assert(string.find("1234567890123456789", "346", 4) == nil)
assert(string.find("1234567890123456789", ".45", -9) == 13)
assert(string.find("abcdefg", "\0", 5, 1) == nil)
assert(string.find("", "") == 1)
assert(string.find("", "", 1) == 1)
assert(not string.find("", "", 2))
assert(string.find('', 'aaa', 1) == nil)
assert(('alo(.)alo'):find('(.)', 1, 1) == 4)
print('+')

assert(string.len("") == 0)
assert(string.len("\0\0\0") == 3)
assert(string.len("1234567890") == 10)

assert(#"" == 0)
assert(#"\0\0\0" == 3)
assert(#"1234567890" == 10)

assert(string.byte("a") == 97)
assert(string.byte("\xe4") > 127)
assert(string.byte(string.char(255)) == 255)
assert(string.byte(string.char(0)) == 0)
assert(string.byte("\0") == 0)
assert(string.byte("\0\0alo\0x", -1) == string.byte('x'))
assert(string.byte("ba", 2) == 97)
assert(string.byte("\n\n", 2, -1) == 10)
assert(string.byte("\n\n", 2, 2) == 10)
assert(string.byte("") == nil)
assert(string.byte("hi", -3) == nil)
assert(string.byte("hi", 3) == nil)
assert(string.byte("hi", 9, 10) == nil)
assert(string.byte("hi", 2, 1) == nil)
assert(string.char() == "")
assert(string.char(0, 255, 0) == "\0\255\0")
assert(string.char(0, string.byte("\xe4"), 0) == "\0\xe4\0")
assert(string.char(string.byte("\xe4l\0óu", 1, -1)) == "\xe4l\0óu")
assert(string.char(string.byte("\xe4l\0óu", 1, 0)) == "")
assert(string.char(string.byte("\xe4l\0óu", -10, 100)) == "\xe4l\0óu")
print('+')

assert(string.upper("ab\0c") == "AB\0C")
assert(string.lower("\0ABCc%$") == "\0abcc%$")
assert(string.rep('teste', 0) == '')
assert(string.rep('tés\00tê', 2) == 'tés\0têtés\000tê')
assert(string.rep('', 10) == '')

-- repetitions with separator
assert(string.rep('teste', 0, 'xuxu') == '')
assert(string.rep('teste', 1, 'xuxu') == 'teste')
assert(string.rep('\1\0\1', 2, '\0\0') == '\1\0\1\0\0\1\0\1')
assert(string.rep('', 10, '.') == string.rep('.', 9))
if not _no32 then
  assert(not pcall(string.rep, "aa", 2^30))
  assert(not pcall(string.rep, "", 2^30, "aa"))
end

assert(string.reverse"" == "")
assert(string.reverse"\0\1\2\3" == "\3\2\1\0")
assert(string.reverse"\0001234" == "4321\0")

for i=0,30 do assert(string.len(string.rep('a', i)) == i) end

assert(type(tostring(nil)) == 'string')
assert(type(tostring(12)) == 'string')
assert(''..12 == '12' and type(12 .. '') == 'string')
assert(string.find(tostring{}, 'table:'))
assert(string.find(tostring(print), 'function:'))
assert(tostring(1234567890123) == '1234567890123')
assert(#tostring('\0') == 1)
assert(tostring(true) == "true")
assert(tostring(false) == "false")
print('+')

x = '"ílo"\n\\'
assert(string.format('%q%s', x, x) == '"\\"ílo\\"\\\n\\\\""ílo"\n\\')
assert(string.format('%q', "\0") == [["\0"]])
assert(load(string.format('return %q', x))() == x)
x = "\0\1\0023\5\0009"
assert(load(string.format('return %q', x))() == x)
assert(string.format("\0%c\0%c%x\0", string.byte("\xe4"), string.byte("b"), 140) ==
              "\0\xe4\0b8c\0")
assert(string.format('') == "")
assert(string.format("%c",34)..string.format("%c",48)..string.format("%c",90)..string.format("%c",100) ==
       string.format("%c%c%c%c", 34, 48, 90, 100))
assert(string.format("%s\0 is not \0%s", 'not be', 'be') == 'not be\0 is not \0be')
assert(string.format("%%%d %010d", 10, 23) == "%10 0000000023")
assert(tonumber(string.format("%f", 10.3)) == 10.3)
x = string.format('"%-50s"', 'a')
assert(#x == 52)
assert(string.sub(x, 1, 4) == '"a  ')

assert(string.format("-%.20s.20s", string.rep("%", 2000)) ==
                     "-"..string.rep("%", 20)..".20s")
assert(string.format('"-%20s.20s"', string.rep("%", 2000)) ==
       string.format("%q", "-"..string.rep("%", 2000)..".20s"))

-- format x tostring
assert(string.format("%s %s", nil, true) == "nil true")
assert(string.format("%s %.4s", false, true) == "false true")
assert(string.format("%.3s %.3s", false, true) == "fal tru")
local m = setmetatable({}, {__tostring = function () return "hello" end})
assert(string.format("%s %.10s", m, m) == "hello hello")


assert(string.format("%x", 0.3) == "0")
assert(string.format("%02x", 0.1) == "00")
assert(string.format("%08X", 2^32 - 1) == "FFFFFFFF")
assert(string.format("%+08d", 2^31 - 1) == "+2147483647")
assert(string.format("%+08d", -2^31) == "-2147483648")


-- longest number that can be formated
assert(string.len(string.format('%99.99f', -1e308)) >= 100)



if not _nolonglong then
  print("testing large numbers for format")
  assert(string.format("%8x", 2^52 - 1) == "fffffffffffff")
  assert(string.format("%d", -1) == "-1")
  assert(tonumber(string.format("%u", 2^62)) == 2^62)
  assert(string.format("%8x", 0xffffffff) == "ffffffff")
  assert(string.format("%8x", 0x7fffffff) == "7fffffff")
  assert(string.format("%d", 2^53) == "9007199254740992")
  assert(string.format("%d", -2^53) == "-9007199254740992")
  assert(string.format("0x%8X", 0x8f000003) == "0x8F000003")
  -- maximum integer that fits both in 64-int and (exact) double
  local x = 2^64 - 2^(64-53)
  assert(x == 0xfffffffffffff800)
  assert(tonumber(string.format("%u", x)) == x)
  assert(tonumber(string.format("0X%x", x)) == x)
  assert(string.format("%x", x) == "fffffffffffff800")
  assert(string.format("%d", x/2) == "9223372036854774784")
  assert(string.format("%d", -x/2) == "-9223372036854774784")
  assert(string.format("%d", -2^63) == "-9223372036854775808")
  assert(string.format("%x", 2^63) == "8000000000000000")
end

if not _noformatA then
  print("testing 'format %a %A'")
  assert(string.format("%.2a", 0.5) == "0x1.00p-1")
  assert(string.format("%A", 0x1fffffffffffff) == "0X1.FFFFFFFFFFFFFP+52")
  assert(string.format("%.4a", -3) == "-0x1.8000p+1")
  assert(tonumber(string.format("%a", -0.1)) == -0.1)
end

-- errors in format

local function check (fmt, msg)
  local s, err = pcall(string.format, fmt, 10)
  assert(not s and string.find(err, msg))
end

local aux = string.rep('0', 600)
check("%100.3d", "too long")
check("%1"..aux..".3d", "too long")
check("%1.100d", "too long")
check("%10.1"..aux.."004d", "too long")
check("%t", "invalid option")
check("%"..aux.."d", "repeated flags")
check("%d %d", "no value")


-- integers out of range
assert(not pcall(string.format, "%d", 2^63))
assert(not pcall(string.format, "%x", 2^64))
assert(not pcall(string.format, "%x", -2^64))
assert(not pcall(string.format, "%x", -1))


assert(load("return 1\n--comentário sem EOL no final")() == 1)


assert(table.concat{} == "")
assert(table.concat({}, 'x') == "")
assert(table.concat({'\0', '\0\1', '\0\1\2'}, '.\0.') == "\0.\0.\0\1.\0.\0\1\2")
local a = {}; for i=1,3000 do a[i] = "xuxu" end
assert(table.concat(a, "123").."123" == string.rep("xuxu123", 3000))
assert(table.concat(a, "b", 20, 20) == "xuxu")
assert(table.concat(a, "", 20, 21) == "xuxuxuxu")
assert(table.concat(a, "x", 22, 21) == "")
assert(table.concat(a, "3", 2999) == "xuxu3xuxu")
if not _no32 then
  assert(table.concat({}, "x", 2^31-1, 2^31-2) == "")
  assert(table.concat({}, "x", -2^31+1, -2^31) == "")
  assert(table.concat({}, "x", 2^31-1, -2^31) == "")
  assert(table.concat({[2^31-1] = "alo"}, "x", 2^31-1, 2^31-1) == "alo")
end

assert(not pcall(table.concat, {"a", "b", {}}))

a = {"a","b","c"}
assert(table.concat(a, ",", 1, 0) == "")
assert(table.concat(a, ",", 1, 1) == "a")
assert(table.concat(a, ",", 1, 2) == "a,b")
assert(table.concat(a, ",", 2) == "b,c")
assert(table.concat(a, ",", 3) == "c")
assert(table.concat(a, ",", 4) == "")

if not _port then

local locales = { "ptb", "ISO-8859-1", "pt_BR" }
local function trylocale (w)
  for i = 1, #locales do
    if os.setlocale(locales[i], w) then return true end
  end
  return false
end

if not trylocale("collate")  then
  print("locale not supported")
else
  assert("alo" < "álo" and "álo" < "amo")
end

if not trylocale("ctype") then
  print("locale not supported")
else
  assert(load("a = 3.4"));  -- parser should not change outside locale
  assert(not load("á = 3.4"));  -- even with errors
  assert(string.gsub("áéíóú", "%a", "x") == "xxxxx")
  assert(string.gsub("áÁéÉ", "%l", "x") == "xÁxÉ")
  assert(string.gsub("áÁéÉ", "%u", "x") == "áxéx")
  assert(string.upper"áÁé{xuxu}ção" == "ÁÁÉ{XUXU}ÇÃO")
end

os.setlocale("C")
assert(os.setlocale() == 'C')
assert(os.setlocale(nil, "numeric") == 'C')

end

print('OK')


