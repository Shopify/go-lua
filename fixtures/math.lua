print("testing numbers and math lib")


-- basic float notation
assert(0e12 == 0 and .0 == 0 and 0. == 0 and .2e2 == 20 and 2.E-1 == 0.2)

do
  local a,b,c = "2", " 3e0 ", " 10  "
  assert(a+b == 5 and -b == -3 and b+"2" == 5 and "10"-c == 0)
  assert(type(a) == 'string' and type(b) == 'string' and type(c) == 'string')
  assert(a == "2" and b == " 3e0 " and c == " 10  " and -c == -"  10 ")
  assert(c%a == 0 and a^b == 08)
  a = 0
  assert(a == -a and 0 == -0)
end

do
  local x = -1
  local mz = 0/x   -- minus zero
  t = {[0] = 10, 20, 30, 40, 50}
  assert(t[mz] == t[0] and t[-0] == t[0])
end

do
  local a,b = math.modf(3.5)
  assert(a == 3 and b == 0.5)
  assert(math.huge > 10e30)
  assert(-math.huge < -10e30)
end

function f(...)
  if select('#', ...) == 1 then
    return (...)
  else
    return "***"
  end
end


-- testing numeric strings

assert("2" + 1 == 3)
assert("2 " + 1 == 3)
assert(" -2 " + 1 == -1)
assert(" -0xa " + 1 == -9)


-- testing 'tonumber'
assert(tonumber{} == nil)
assert(tonumber'+0.01' == 1/100 and tonumber'+.01' == 0.01 and
       tonumber'.01' == 0.01    and tonumber'-1.' == -1 and
       tonumber'+1.' == 1)
assert(tonumber'+ 0.01' == nil and tonumber'+.e1' == nil and
       tonumber'1e' == nil     and tonumber'1.0e+' == nil and
       tonumber'.' == nil)
assert(tonumber('-012') == -010-2)
assert(tonumber('-1.2e2') == - - -120)

assert(tonumber("0xffffffffffff") == 2^(4*12) - 1)
assert(tonumber("0x"..string.rep("f", 150)) == 2^(4*150) - 1)
assert(tonumber('0x3.' .. string.rep('0', 100)) == 3)
assert(tonumber('0x0.' .. string.rep('0', 150).."1") == 2^(-4*151))

-- testing 'tonumber' with base
assert(tonumber('  001010  ', 2) == 10)
assert(tonumber('  001010  ', 10) == 001010)
assert(tonumber('  -1010  ', 2) == -10)
assert(tonumber('10', 36) == 36)
assert(tonumber('  -10  ', 36) == -36)
assert(tonumber('  +1Z  ', 36) == 36 + 35)
assert(tonumber('  -1z  ', 36) == -36 + -35)
assert(tonumber('-fFfa', 16) == -(10+(16*(15+(16*(15+(16*15)))))))
assert(tonumber(string.rep('1', 42), 2) + 1 == 2^42)
assert(tonumber(string.rep('1', 34), 2) + 1 == 2^34)
assert(tonumber('ffffFFFF', 16)+1 == 2^32)
assert(tonumber('0ffffFFFF', 16)+1 == 2^32)
assert(tonumber('-0ffffffFFFF', 16) - 1 == -2^40)
for i = 2,36 do
  assert(tonumber('\t10000000000\t', i) == i^10)
end

-- testing 'tonumber' fo invalid formats
assert(f(tonumber('fFfa', 15)) == nil)
assert(f(tonumber('099', 8)) == nil)
assert(f(tonumber('1\0', 2)) == nil)
assert(f(tonumber('', 8)) == nil)
assert(f(tonumber('  ', 9)) == nil)
assert(f(tonumber('  ', 9)) == nil)
assert(f(tonumber('0xf', 10)) == nil)

assert(f(tonumber('inf')) == nil)
assert(f(tonumber(' INF ')) == nil)
assert(f(tonumber('Nan')) == nil)
assert(f(tonumber('nan')) == nil)

assert(f(tonumber('  ')) == nil)
assert(f(tonumber('')) == nil)
assert(f(tonumber('1  a')) == nil)
assert(f(tonumber('1\0')) == nil)
assert(f(tonumber('1 \0')) == nil)
assert(f(tonumber('1\0 ')) == nil)
assert(f(tonumber('e1')) == nil)
assert(f(tonumber('e  1')) == nil)
assert(f(tonumber(' 3.4.5 ')) == nil)


-- testing 'tonumber' for invalid hexadecimal formats

assert(tonumber('0x') == nil)
assert(tonumber('x') == nil)
assert(tonumber('x3') == nil)
assert(tonumber('00x2') == nil)
assert(tonumber('0x 2') == nil)
assert(tonumber('0 x2') == nil)
assert(tonumber('23x') == nil)
assert(tonumber('- 0xaa') == nil)


-- testing hexadecimal numerals

assert(0x10 == 16 and 0xfff == 2^12 - 1 and 0XFB == 251)
assert(0x0p12 == 0 and 0x.0p-3 == 0)
assert(0xFFFFFFFF == 2^32 - 1)
assert(tonumber('+0x2') == 2)
assert(tonumber('-0xaA') == -170)
assert(tonumber('-0xffFFFfff') == -2^32 + 1)

-- possible confusion with decimal exponent
assert(0E+1 == 0 and 0xE+1 == 15 and 0xe-1 == 13)


-- floating hexas

assert(tonumber('  0x2.5  ') == 0x25/16)
assert(tonumber('  -0x2.5  ') == -0x25/16)
assert(tonumber('  +0x0.51p+8  ') == 0x51)
assert(tonumber('0x0.51p') == nil)
assert(tonumber('0x5p+-2') == nil)
assert(0x.FfffFFFF == 1 - '0x.00000001')
assert('0xA.a' + 0 == 10 + 10/16)
assert(0xa.aP4 == 0XAA)
assert(0x4P-2 == 1)
assert(0x1.1 == '0x1.' + '+0x.1')


assert(1.1 == 1.+.1)
assert(100.0 == 1E2 and .01 == 1e-2)
assert(1111111111111111-1111111111111110== 1000.00e-03)
--     1234567890123456
assert(1.1 == '1.'+'.1')
assert('1111111111111111'-'1111111111111110' == tonumber"  +0.001e+3 \n\t")

function eq (a,b,limit)
  if not limit then limit = 10E-10 end
  return math.abs(a-b) <= limit
end

assert(0.1e-30 > 0.9E-31 and 0.9E30 < 0.1e31)

assert(0.123456 > 0.123455)

assert(tonumber('+1.23E18') == 1.23*10^18)

-- testing order operators
assert(not(1<1) and (1<2) and not(2<1))
assert(not('a'<'a') and ('a'<'b') and not('b'<'a'))
assert((1<=1) and (1<=2) and not(2<=1))
assert(('a'<='a') and ('a'<='b') and not('b'<='a'))
assert(not(1>1) and not(1>2) and (2>1))
assert(not('a'>'a') and not('a'>'b') and ('b'>'a'))
assert((1>=1) and not(1>=2) and (2>=1))
assert(('a'>='a') and not('a'>='b') and ('b'>='a'))

-- testing mod operator
assert(-4%3 == 2)
assert(4%-3 == -2)
assert(math.pi - math.pi % 1 == 3)
assert(math.pi - math.pi % 0.001 == 3.141)

local function testbit(a, n)
  return a/2^n % 2 >= 1
end

assert(eq(math.sin(-9.8)^2 + math.cos(-9.8)^2, 1))
assert(eq(math.tan(math.pi/4), 1))
assert(eq(math.sin(math.pi/2), 1) and eq(math.cos(math.pi/2), 0))
assert(eq(math.atan(1), math.pi/4) and eq(math.acos(0), math.pi/2) and
       eq(math.asin(1), math.pi/2))
assert(eq(math.deg(math.pi/2), 90) and eq(math.rad(90), math.pi/2))
assert(math.abs(-10) == 10)
assert(eq(math.atan2(1,0), math.pi/2))
assert(math.ceil(4.5) == 5.0)
assert(math.floor(4.5) == 4.0)
assert(math.fmod(10,3) == 1)
assert(eq(math.sqrt(10)^2, 10))
assert(eq(math.log(2, 10), math.log(2)/math.log(10)))
assert(eq(math.log(2, 2), 1))
assert(eq(math.log(9, 3), 2))
assert(eq(math.exp(0), 1))
assert(eq(math.sin(10), math.sin(10%(2*math.pi))))
local v,e = math.frexp(math.pi)
assert(eq(math.ldexp(v,e), math.pi))

assert(eq(math.tanh(3.5), math.sinh(3.5)/math.cosh(3.5)))

assert(tonumber(' 1.3e-2 ') == 1.3e-2)
assert(tonumber(' -1.00000000000001 ') == -1.00000000000001)

-- testing constant limits
-- 2^23 = 8388608
assert(8388609 + -8388609 == 0)
assert(8388608 + -8388608 == 0)
assert(8388607 + -8388607 == 0)

-- testing implicit convertions

local a,b = '10', '20'
assert(a*b == 200 and a+b == 30 and a-b == -10 and a/b == 0.5 and -b == -20)
assert(a == '10' and b == '20')


if not _port then
  print("testing -0 and NaN")
  local mz, z = -0, 0
  assert(mz == z)
  assert(1/mz < 0 and 0 < 1/z)
  local a = {[mz] = 1}
  assert(a[z] == 1 and a[mz] == 1)
  local inf = math.huge * 2 + 1
  mz, z = -1/inf, 1/inf
  assert(mz == z)
  assert(1/mz < 0 and 0 < 1/z)
  local NaN = inf - inf
  assert(NaN ~= NaN)
  assert(not (NaN < NaN))
  assert(not (NaN <= NaN))
  assert(not (NaN > NaN))
  assert(not (NaN >= NaN))
  assert(not (0 < NaN) and not (NaN < 0))
  local NaN1 = 0/0
  assert(NaN ~= NaN1 and not (NaN <= NaN1) and not (NaN1 <= NaN))
  local a = {}
  assert(not pcall(function () a[NaN] = 1 end))
  assert(a[NaN] == nil)
  a[1] = 1
  assert(not pcall(function () a[NaN] = 1 end))
  assert(a[NaN] == nil)
  -- string with same binary representation as 0.0 (may create problems
  -- for constant manipulation in the pre-compiler)
  local a1, a2, a3, a4, a5 = 0, 0, "\0\0\0\0\0\0\0\0", 0, "\0\0\0\0\0\0\0\0"
  assert(a1 == a2 and a2 == a4 and a1 ~= a3)
  assert(a3 == a5)
end


if not _port then
  print("testing 'math.random'")
  math.randomseed(0)

  local function aux (x1, x2, p)
    local Max = -math.huge
    local Min = math.huge
    for i = 0, 20000 do
      local t = math.random(table.unpack(p))
      Max = math.max(Max, t)
      Min = math.min(Min, t)
      if eq(Max, x2, 0.001) and eq(Min, x1, 0.001) then
        goto ok
      end
    end
    -- loop ended without satisfing condition
    assert(false)
   ::ok::
    assert(x1 <= Min and Max<=x2)
  end

  aux(0, 1, {})
  aux(-10, 0, {-10,0})
end

for i=1,10 do
  local t = math.random(5)
  assert(1 <= t and t <= 5)
end


print('OK')
