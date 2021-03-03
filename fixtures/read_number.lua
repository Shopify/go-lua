file = io.open("fixtures/number.txt", "r")
print(file:read("*n"))
file:close()
