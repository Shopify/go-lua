package lua

// Test assumptions about how Go works

import (
	"math"
	"strconv"
	"testing"
	"unicode"
)

func TestStringCompare(t *testing.T) {
	s1 := "hello\x00world"
	s2 := "hello\x00sweet"
	if s1 <= s2 {
		t.Error("s1 <= s2")
	}
}

func TestStringLength(t *testing.T) {
	s := "hello\x00world"
	if len(s) != 11 {
		t.Error("go doesn't count embedded nulls in string length")
	}
}

func TestIsControl(t *testing.T) {
	t.Skip()
	for i := 0; i < 256; i++ {
		control := i < 0x20 || i == 0x7f
		if lib := unicode.Is(unicode.Cc, rune(i)); control != lib {
			t.Errorf("%x: is control? %s", i, lib)
		}
	}
}

func TestReslicing(t *testing.T) {
	a := [5]int{0, 1, 2, 3, 4}
	s := a[:0]
	if cap(s) != cap(a) {
		t.Error("cap(s) != cap(a)")
	}
	if len(s) != 0 {
		t.Error("len(s) != 0")
	}
	s = a[1:3]
	if cap(s) == len(s) {
		t.Error("cap(s) == len(s)")
	}
	s = s[:cap(s)]
	if cap(s) != len(s) {
		t.Error("cap(s) != len(s)")
	}
}

func TestPow(t *testing.T) {
	// if a, b := math.Pow(10.0, 33.0), 1.0e33; a != b {
	// 	t.Errorf("%v != %v\n", a, b)
	// }
	if a, b := math.Pow10(33), 1.0e33; a != b {
		t.Errorf("%v != %v\n", a, b)
	}
}

func TestZero(t *testing.T) {
	if 0.0 != -0.0 {
		t.Error("0.0 == -0.0")
	}
}

func TestParseFloat(t *testing.T) {
	if f, err := strconv.ParseFloat("inf", 64); err != nil {
		t.Error("ParseFloat('inf', 64) == ", f, err)
	}
}

func TestUnsigned(t *testing.T) {
	n := -1.0
	const supUnsigned = float64(^uint32(0)) + 1
	if x := math.Floor(n / supUnsigned); x != -1.0 {
		t.Error("math.Floor(-1/supUnsigned) == ", x)
	}
	if x := math.Floor(n/supUnsigned) * supUnsigned; x != -4294967296.0 {
		t.Error("math.Floor(n/supUnsigned)*supUnsigned == ", x)
	}
	if x := n - math.Floor(n/supUnsigned)*supUnsigned; x != 4294967295.0 {
		t.Error("n-math.Floor(n/supUnsigned)*supUnsigned == ", x)
	}
	if x := uint(n - math.Floor(n/supUnsigned)*supUnsigned); x != 4294967295 {
		t.Error("uint(n-math.Floor(n/supUnsigned)*supUnsigned) == ", x)
	}
}
