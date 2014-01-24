function fib (n)
	if n == 0 then
		return 0
	elseif n == 1 then
		return 1
	end
	return fib(n-1) + fib(n-2)
end

function fibr (n0, n1, c)
	if c == 0 then
		return n0
	elseif c == 1 then
		return n1
	end
	return fibr(n1, n0+n1, c-1)
end

function fibl (n)
	if n == 0 then
		return 0
	elseif n == 1 then
		return 1
	end
	local n0, n1 = 0, 1
	for i = n, 2, -1 do
		local tmp = n0 + n1
		n0 = n1
		n1 = tmp
	end
	return n1
end

print(fib(20))
print(fibr(0, 1, 20))
print(fibl(20))
