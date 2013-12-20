package lua

import (
	"math"
	"strconv"
	"fmt"
)

type value interface{}
type stackIndex int
type float8 int

func printValue(v value) {
	switch v := v.(type) {
	case *table:
		print("table ", v)
	case string:
		print("'", v, "'")
	case float64:
		print(v)
	case *luaClosure:
		print("closure ", v)
	case nil:
		print("nil")
	default:
		print("unknown ", v)
	}
}

func isFalse(s value) bool {
	b, isBool := s.(bool)
	return s == nil || (isBool && !b)
}

type localVariable struct {
	name           string
	startPC, endPC pc
}

type userData struct {
	metaTable, env *table
}

type upValueDesc struct {
	name    string
	isLocal bool
	index   byte
}

type stackLocation struct {
	state *state
	index stackIndex
}

type prototype struct {
	constants                    []value
	code                         []instruction
	prototypes                   []prototype
	lineInfo                     []int32
	localVariables               []localVariable
	upValues                     []upValueDesc
	cache                        *luaClosure
	source                       string
	lineDefined, lastLineDefined int
	parameterCount, maxStackSize byte
	isVarArg                     bool
}

func intFromFloat8(x float8) int {
	e := (x >> 3) & 0x1f
	if e == 0 {
		return int(x)
	}
	return int((x&7)+8) << uint(e-1)
}

func arith(op tm, v1, v2 float64) float64 {
	switch op {
	case tmAdd:
		return v1 + v2
	case tmSub:
		return v1 - v2
	case tmMul:
		return v1 * v2
	case tmDiv:
		return v1 / v2
	case tmMod:
		return math.Mod(v1, v2)
	case tmPow:
		return math.Pow(v1, v2)
	case tmUnaryMinus:
		return -v1
	}
	panic(op)
}

func toNumber(r value) (float64, bool) {
	if v, ok := r.(float64); ok {
		return v, true
	}
	if s, ok := r.(string); ok {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v, true
		}
	}
	return 0.0, false
}

func numberToString(f float64) string {
	return fmt.Sprintf("%.14g", f)
}

func toString(r value) (string, bool) {
	if v, ok := r.(float64); ok {
		return numberToString(v), true
	}
	return "", false
}

func pairAsNumbers(p1, p2 value) (float64, float64, bool) {
	f1, ok1 := p1.(float64)
	f2, ok2 := p2.(float64)
	return f1, f2, ok1 && ok2
}

func pairAsStrings(p1, p2 value) (string, string, bool) {
	s1, ok1 := p1.(string)
	s2, ok2 := p2.(string)
	return s1, s2, ok1 && ok2
}
