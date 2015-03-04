function fibt(n0, n1, c)
	if c == 0 then
		return n0
	elseif c == 1 then
		return n1
	end
	return fibt(n1, n0+n1, c-1)
end

function fib(n)
  fibt(0, 1, n)
end

fib(1000000)
