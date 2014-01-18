package math

import (
	"github.com/Shopify/go-lua"
	"math"
	"math/rand"
)

const radiansPerDegree = math.Pi / 180.0

func unaryOp(f func(float64) float64) lua.Function {
	return func(l *lua.State) int {
		lua.PushNumber(l, f(lua.CheckNumber(l, 1)))
		return 1
	}
}

func binaryOp(f func(float64, float64) float64) lua.Function {
	return func(l *lua.State) int {
		lua.PushNumber(l, f(lua.CheckNumber(l, 1), lua.CheckNumber(l, 2)))
		return 1
	}
}

func modf(l *lua.State) int {
	i, f := math.Modf(lua.CheckNumber(l, 1))
	lua.PushNumber(l, i)
	lua.PushNumber(l, f)
	return 2
}

func log(l *lua.State) int {
	x := lua.CheckNumber(l, 1)
	if lua.IsNoneOrNil(l, 2) {
		lua.PushNumber(l, math.Log(x))
	} else if base := lua.CheckNumber(l, 2); base == 10.0 {
		lua.PushNumber(l, math.Log10(x))
	} else {
		lua.PushNumber(l, math.Log(x)/math.Log(base))
	}
	return 1
}

func frexp(l *lua.State) int {
	f, e := math.Frexp(lua.CheckNumber(l, 1))
	lua.PushNumber(l, f)
	lua.PushInteger(l, e)
	return 2
}

func ldexp(l *lua.State) int {
	x, e := lua.CheckNumber(l, 1), lua.CheckInteger(l, 2)
	lua.PushNumber(l, math.Ldexp(x, e))
	return 1
}

func reduce(f func(float64, float64) float64) lua.Function {
	return func(l *lua.State) int {
		n := lua.Top(l) // number of arguments
		v := lua.CheckNumber(l, 1)
		for i := 2; i <= n; i++ {
			v = f(v, lua.CheckNumber(l, i))
		}
		lua.PushNumber(l, v)
		return 1
	}
}

func random(l *lua.State) int {
	r := rand.Float64()
	switch lua.Top(l) {
	case 0: // no arguments
		lua.PushNumber(l, r)
	case 1: // upper limit only
		u := lua.CheckNumber(l, 1)
		lua.ArgumentCheck(l, 1.0 <= u, 1, "interval is empty")
		lua.PushNumber(l, math.Floor(r*u)+1.0) // [1, u]
	case 2: // lower and upper limits
		lo, u := lua.CheckNumber(l, 1), lua.CheckNumber(l, 2)
		lua.ArgumentCheck(l, lo <= u, 2, "interval is empty")
		lua.PushNumber(l, math.Floor(r*(u-lo+1))+lo) // [lo, u]
	default:
		lua.Errorf(l, "wrong number of arguments")
	}
	return 1
}

func randomseed(l *lua.State) int {
	rand.Seed(int64(lua.CheckUnsigned(l, 1)))
	rand.Float64() // discard first value to avoid undesirable correlations
	return 0
}

var mathLibrary = []lua.RegistryFunction{
	{"abs", unaryOp(math.Abs)},
	{"acos", unaryOp(math.Acos)},
	{"asin", unaryOp(math.Asin)},
	{"atan2", binaryOp(math.Atan2)},
	{"atan", unaryOp(math.Atan)},
	{"ceil", unaryOp(math.Ceil)},
	{"cosh", unaryOp(math.Cosh)},
	{"cos", unaryOp(math.Cos)},
	{"deg", unaryOp(func(x float64) float64 { return x / radiansPerDegree })},
	{"exp", unaryOp(math.Exp)},
	{"floor", unaryOp(math.Floor)},
	{"fmod", binaryOp(math.Mod)},
	{"frexp", frexp},
	{"ldexp", ldexp},
	{"log", log},
	{"max", reduce(math.Max)},
	{"min", reduce(math.Min)},
	{"modf", modf},
	{"pow", binaryOp(math.Pow)},
	{"rad", unaryOp(func(x float64) float64 { return x * radiansPerDegree })},
	{"random", random},
	{"randomseed", randomseed},
	{"sinh", unaryOp(math.Sinh)},
	{"sin", unaryOp(math.Sin)},
	{"sqrt", unaryOp(math.Sqrt)},
	{"tanh", unaryOp(math.Tanh)},
	{"tan", unaryOp(math.Tan)},
}

func Open(l *lua.State) int {
	lua.NewLibrary(l, mathLibrary)
	lua.PushNumber(l, 3.1415926535897932384626433832795) // TODO use math.Pi instead? Values differ.
	lua.SetField(l, -2, "pi")
	lua.PushNumber(l, math.MaxFloat64)
	lua.SetField(l, -2, "huge")
	return 1
}
