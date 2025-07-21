file = io.open("fixtures/io.txt", "r")
print(file:read("*a"))
file:close()
