package lua

import "testing"

func TestFormatLargeIntegers(t *testing.T) {
	testString(t, `
		assert(string.format("%012d", 34567890121111) == "34567890121111")
		assert(string.format("%i", -34567890121111) == "-34567890121111")
		assert(string.format("%u", 34567890121111) == "34567890121111")
		assert(string.format("%o", 34567890121111) == "767016663730627")
		assert(string.format("%x", 34567890121111) == "1f7076cfb197")
		assert(string.format("%X", 34567890121111) == "1F7076CFB197")
	`)
}
