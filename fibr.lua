function fibr(n)
	if n == 0 then
		return 0
	elseif n == 1 then
		return 1
	end
	return fibr(n-1) + fibr(n-2)
end

print(fibr(30))
