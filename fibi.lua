function fibi(n)
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

print(fibi(1000000))
