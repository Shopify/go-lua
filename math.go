package lua

import (
	"math"
	"math/rand"
)

const radiansPerDegree = math.Pi / 180.0

func mathUnaryOp(f func(float64) float64) Function {
	return func(l *State) int {
		l.PushNumber(f(CheckNumber(l, 1)))
		return 1
	}
}

func mathBinaryOp(f func(float64, float64) float64) Function {
	return func(l *State) int {
		l.PushNumber(f(CheckNumber(l, 1), CheckNumber(l, 2)))
		return 1
	}
}

func reduce(f func(float64, float64) float64) Function {
	return func(l *State) int {
		n := l.Top() // number of arguments
		v := CheckNumber(l, 1)
		for i := 2; i <= n; i++ {
			v = f(v, CheckNumber(l, i))
		}
		l.PushNumber(v)
		return 1
	}
}

var mathLibrary = []RegistryFunction{
	{"abs", mathUnaryOp(math.Abs)},
	{"acos", mathUnaryOp(math.Acos)},
	{"asin", mathUnaryOp(math.Asin)},
	{"atan2", mathBinaryOp(math.Atan2)},
	{"atan", mathUnaryOp(math.Atan)},
	{"ceil", mathUnaryOp(math.Ceil)},
	{"cosh", mathUnaryOp(math.Cosh)},
	{"cos", mathUnaryOp(math.Cos)},
	{"deg", mathUnaryOp(func(x float64) float64 { return x / radiansPerDegree })},
	{"exp", mathUnaryOp(math.Exp)},
	{"floor", mathUnaryOp(math.Floor)},
	{"fmod", mathBinaryOp(math.Mod)},
	{"frexp", func(l *State) int {
		f, e := math.Frexp(CheckNumber(l, 1))
		l.PushNumber(f)
		l.PushInteger(e)
		return 2
	}},
	{"ldexp", func(l *State) int {
		x, e := CheckNumber(l, 1), CheckInteger(l, 2)
		l.PushNumber(math.Ldexp(x, e))
		return 1
	}},
	{"log", func(l *State) int {
		x := CheckNumber(l, 1)
		if l.IsNoneOrNil(2) {
			l.PushNumber(math.Log(x))
		} else if base := CheckNumber(l, 2); base == 10.0 {
			l.PushNumber(math.Log10(x))
		} else {
			l.PushNumber(math.Log(x) / math.Log(base))
		}
		return 1
	}},
	{"max", reduce(math.Max)},
	{"min", reduce(math.Min)},
	{"modf", func(l *State) int {
		i, f := math.Modf(CheckNumber(l, 1))
		l.PushNumber(i)
		l.PushNumber(f)
		return 2
	}},
	{"pow", mathBinaryOp(math.Pow)},
	{"rad", mathUnaryOp(func(x float64) float64 { return x * radiansPerDegree })},
	{"random", func(l *State) int {
		r := rand.Float64()
		switch l.Top() {
		case 0: // no arguments
			l.PushNumber(r)
		case 1: // upper limit only
			u := CheckNumber(l, 1)
			ArgumentCheck(l, 1.0 <= u, 1, "interval is empty")
			l.PushNumber(math.Floor(r*u) + 1.0) // [1, u]
		case 2: // lower and upper limits
			lo, u := CheckNumber(l, 1), CheckNumber(l, 2)
			ArgumentCheck(l, lo <= u, 2, "interval is empty")
			l.PushNumber(math.Floor(r*(u-lo+1)) + lo) // [lo, u]
		default:
			Errorf(l, "wrong number of arguments")
		}
		return 1
	}},
	{"randomseed", func(l *State) int {
		rand.Seed(int64(CheckUnsigned(l, 1)))
		rand.Float64() // discard first value to avoid undesirable correlations
		return 0
	}},
	{"sinh", mathUnaryOp(math.Sinh)},
	{"sin", mathUnaryOp(math.Sin)},
	{"sqrt", mathUnaryOp(math.Sqrt)},
	{"tanh", mathUnaryOp(math.Tanh)},
	{"tan", mathUnaryOp(math.Tan)},
}

// MathOpen opens the math library. Usually passed to Require.
func MathOpen(l *State) int {
	NewLibrary(l, mathLibrary)
	l.PushNumber(3.1415926535897932384626433832795) // TODO use math.Pi instead? Values differ.
	l.SetField(-2, "pi")
	l.PushNumber(math.MaxFloat64)
	l.SetField(-2, "huge")
	return 1
}
