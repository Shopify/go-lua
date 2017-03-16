package lua

import (
	"math"
)

const bitCount = 32

func trim(x uint) uint { return x & math.MaxUint32 }
func mask(n uint) uint { return ^(math.MaxUint32 << n) }

func shift(l *State, r uint, i int) int {
	if i < 0 {
		if i, r = -i, trim(r); i >= bitCount {
			r = 0
		} else {
			r >>= uint(i)
		}
	} else {
		if i >= bitCount {
			r = 0
		} else {
			r <<= uint(i)
		}
		r = trim(r)
	}
	l.PushUnsigned(r)
	return 1
}

func rotate(l *State, i int) int {
	r := trim(CheckUnsigned(l, 1))
	if i &= bitCount - 1; i != 0 {
		r = trim((r << uint(i)) | (r >> uint(bitCount-i)))
	}
	l.PushUnsigned(r)
	return 1
}

func bitOp(l *State, init uint, f func(a, b uint) uint) uint {
	r := init
	for i, n := 1, l.Top(); i <= n; i++ {
		r = f(r, CheckUnsigned(l, i))
	}
	return trim(r)
}

func andHelper(l *State) uint {
	x := bitOp(l, ^uint(0), func(a, b uint) uint { return a & b })
	return x
}

func fieldArguments(l *State, fieldIndex int) (uint, uint) {
	f, w := CheckInteger(l, fieldIndex), OptInteger(l, fieldIndex+1, 1)
	ArgumentCheck(l, 0 <= f, fieldIndex, "field cannot be negative")
	ArgumentCheck(l, 0 < w, fieldIndex+1, "width must be positive")
	if f+w > bitCount {
		Errorf(l, "trying to access non-existent bits")
	}
	return uint(f), uint(w)
}

var bitLibrary = []RegistryFunction{
	{"arshift", func(l *State) int {
		r, i := CheckUnsigned(l, 1), CheckInteger(l, 2)
		if i < 0 || 0 == (r&(1<<(bitCount-1))) {
			return shift(l, r, -i)
		}

		if i >= bitCount {
			r = math.MaxUint32
		} else {
			r = trim((r >> uint(i)) | ^(math.MaxUint32 >> uint(i)))
		}
		l.PushUnsigned(r)
		return 1
	}},
	{"band", func(l *State) int { l.PushUnsigned(andHelper(l)); return 1 }},
	{"bnot", func(l *State) int { l.PushUnsigned(trim(^CheckUnsigned(l, 1))); return 1 }},
	{"bor", func(l *State) int { l.PushUnsigned(bitOp(l, 0, func(a, b uint) uint { return a | b })); return 1 }},
	{"bxor", func(l *State) int { l.PushUnsigned(bitOp(l, 0, func(a, b uint) uint { return a ^ b })); return 1 }},
	{"btest", func(l *State) int { l.PushBoolean(andHelper(l) != 0); return 1 }},
	{"extract", func(l *State) int {
		r := CheckUnsigned(l, 1)
		f, w := fieldArguments(l, 2)
		l.PushUnsigned((r >> f) & mask(w))
		return 1
	}},
	{"lrotate", func(l *State) int { return rotate(l, CheckInteger(l, 2)) }},
	{"lshift", func(l *State) int { return shift(l, CheckUnsigned(l, 1), CheckInteger(l, 2)) }},
	{"replace", func(l *State) int {
		r, v := CheckUnsigned(l, 1), CheckUnsigned(l, 2)
		f, w := fieldArguments(l, 3)
		m := mask(w)
		v &= m
		l.PushUnsigned((r & ^(m << f)) | (v << f))
		return 1
	}},
	{"rrotate", func(l *State) int { return rotate(l, -CheckInteger(l, 2)) }},
	{"rshift", func(l *State) int { return shift(l, CheckUnsigned(l, 1), -CheckInteger(l, 2)) }},
}

// Bit32Open opens the bit32 library. Usually passed to Require.
func Bit32Open(l *State) int {
	NewLibrary(l, bitLibrary)
	return 1
}
