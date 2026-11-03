package lua

import (
	"fmt"
	"math"
	"testing"
)

func TestPowerConsistentAcrossEvaluationPaths(t *testing.T) {
	engines := []struct {
		name string
		run  func(*State)
	}{
		{"function table", (*State).executeFunctionTable},
		{"switch", (*State).executeSwitch},
	}
	cases := []struct {
		base, exponent             string
		baseNumber, exponentNumber float64
	}{
		{"10", "-33", 10, -33},
		{"10", "33", 10, 33},
		{"10", "100", 10, 100},
		{"10", "308", 10, 308},
		{"10", "309", 10, 309},
		{"10", "-323", 10, -323},
		{"10", "-324", 10, -324},
		{"-2", "3", -2, 3},
		{"-2", "-3", -2, -3},
		{"-2", "0.5", -2, 0.5},
		{"0", "0", 0, 0},
		{"0", "-3", 0, -3},
		{"-0.0", "3", math.Copysign(0, -1), 3},
		{"-0.0", "-3", math.Copysign(0, -1), -3},
		{"-0.0", "2", math.Copysign(0, -1), 2},
		{"-0.0", "-2", math.Copysign(0, -1), -2},
		{"2", "0.5", 2, 0.5},
	}
	for _, engine := range engines {
		for _, tc := range cases {
			t.Run(engine.name+"/"+tc.base+"^"+tc.exponent, func(t *testing.T) {
				l := NewState()
				OpenLibraries(l)
				source := fmt.Sprintf(`
local base, exponent = (%s), (%s)
local folded = (%s)^(%s)
local runtime = base^exponent
local coerced = %q^%q
local library = math.pow(base, exponent)
local keyMatches = true
if folded == folded then
  local keys = {[folded] = "found"}
  keyMatches = keys[runtime] == "found" and keys[coerced] == "found" and keys[library] == "found"
end
return folded, runtime, coerced, library, keyMatches
`, tc.base, tc.exponent, tc.base, tc.exponent, tc.base, tc.exponent)
				if err := LoadString(l, source); err != nil {
					t.Fatal(err)
				}
				if err := l.protectedCall(func() {
					if l.preCall(l.top-1, MultipleReturns) {
						t.Fatal("expected a Lua function")
					}
					engine.run(l)
				}, l.top-1, 0); err != nil {
					t.Fatal(err)
				}
				// Use the runtime's numeric primitive, not a decimal approximation
				// or the result from a different host math library.
				want := math.Pow(tc.baseNumber, tc.exponentNumber)
				for index, name := range []string{"folded", "runtime", "coerced", "math.pow"} {
					got, ok := l.ToNumber(index + 1)
					if !ok || !samePowerNumber(got, want) {
						t.Errorf("%s = %g (%016x), want %g (%016x)", name, got, math.Float64bits(got), want, math.Float64bits(want))
					}
				}
				if !l.ToBoolean(5) {
					t.Error("folded and runtime powers did not address the same table key")
				}
				for _, coerce := range []bool{false, true} {
					api := NewState()
					if coerce {
						api.PushString(tc.base)
						api.PushString(tc.exponent)
					} else {
						api.PushNumber(tc.baseNumber)
						api.PushNumber(tc.exponentNumber)
					}
					api.Arith(OpPow)
					if got, ok := api.ToNumber(-1); !ok || !samePowerNumber(got, want) {
						t.Errorf("Arith(OpPow), coerce=%v: %g, want %g", coerce, got, want)
					}
				}
			})
		}
	}
}

func samePowerNumber(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || math.IsNaN(a) && math.IsNaN(b)
}
