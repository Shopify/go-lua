file = io.open("fixtures/io.txt", "r")
print(file:read("*l"))
print(file:read("*l"))
file:close()
