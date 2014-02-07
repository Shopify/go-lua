package lua

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strconv"
)

type value interface{}
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
		print(fmt.Sprintf("closure %s:%d %v", v.prototype.source, v.prototype.lineDefined, v))
	case *goClosure:
		print("go closure ", v)
	case Function:
		pc := reflect.ValueOf(v).Pointer()
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc)
		print(fmt.Sprintf("go function %s %s:%d", f.Name(), file, line))
	case *userData:
		print("userdata ", v)
	case nil:
		print("nil")
	default:
		print("unknown ", v)
	}
}

func printStack(s []value) {
	println("stack (len: ", len(s), ", cap: ", cap(s), "):")
	for i, v := range s {
		print("  ", i, ": ")
		printValue(v)
		println()
	}
}

func isFalse(s value) bool {
	b, isBool := s.(bool)
	return s == nil || isBool && !b
}

type localVariable struct {
	name           string
	startPC, endPC pc
}

type userData struct {
	metaTable, env *table
	data           interface{}
}

type upValueDesc struct {
	name    string
	isLocal bool
	index   int
}

type stackLocation struct {
	state *State
	index int
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
	parameterCount, maxStackSize int
	isVarArg                     bool
}

// Converts an integer to a "floating point byte", represented as
// (eeeeexxx), where the real value is (1xxx) * 2^(eeeee - 1) if
// eeeee != 0 and (xxx) otherwise.
func float8FromInt(x int) float8 {
	if x < 8 {
		return float8(x)
	}
	e := 0
	for ; x >= 0x10; e++ {
		x = (x + 1) >> 1
	}
	return float8(((e + 1) << 3) | (x - 8))
}

func intFromFloat8(x float8) int {
	e := x >> 3 & 0x1f
	if e == 0 {
		return int(x)
	}
	return int(x&7+8) << uint(e-1)
}

func arith(op int, v1, v2 float64) float64 {
	switch op {
	case OpAdd:
		return v1 + v2
	case OpSub:
		return v1 - v2
	case OpMul:
		return v1 * v2
	case OpDiv:
		return v1 / v2
	case OpMod:
		return math.Mod(v1, v2)
	case OpPow:
		return math.Pow(v1, v2)
	case OpUnaryMinus:
		return -v1
	}

	panic(fmt.Sprintf("not an arithmetic op code (%d)", op))
}

func toNumber(r value) (float64, bool) {
	if v, ok := r.(float64); ok {
		return v, true
	}
	if s, ok := r.(string); ok {
		if v, err := strconv.ParseFloat(s, 64); err == nil { // TODO handle hexadecimal floats
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
