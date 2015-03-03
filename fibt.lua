function fibt(n0, n1, c)
	if c == 0 then
		return n0
	elseif c == 1 then
		return n1
	end
	return fibt(n1, n0+n1, c-1)
end

print(fibt(0, 1, 1000000))
