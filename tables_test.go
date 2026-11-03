package lua

import (
	"fmt"
	"testing"
)

func TestNextUsesRawKeyEquality(t *testing.T) {
	for _, kind := range []string{"table", "userdata"} {
		t.Run(kind, func(t *testing.T) {
			l := NewState()
			OpenLibraries(l)
			if err := DoString(l, `return function(a, b, c)
				local mt = {__eq = function() error("next must not invoke __eq") end}
				debug.setmetatable(a, mt)
				debug.setmetatable(b, mt)
				debug.setmetatable(c, mt)
				local t = {[a] = 1, [b] = 2, [c] = 3}
				local seen, count = {}, 0
				for k, v in pairs(t) do
					assert(not seen[k])
					seen[k] = true
					assert(t[k] == v)
					count = count + 1
				end
				assert(count == 3)
			end`); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if kind == "table" {
					l.CreateTable(0, 0)
				} else {
					l.PushUserData(i)
				}
			}
			if err := l.ProtectedCall(3, 0, 0); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNextHashTableChanges(t *testing.T) {
	tests := []struct {
		name, program string
	}{
		{"interleaved", `
			local t = {a = 1, b = 2, c = 3, [false] = 4, [100] = 5, 6, 7}
			local outer, count = {}, 0
			for k, v in pairs(t) do
				assert(not outer[k])
				outer[k] = true
				local inner, n = {}, 0
				for x, y in pairs(t) do
					assert(not inner[x])
					inner[x] = true
					assert(t[x] == y)
					n = n + 1
				end
				assert(n == 7)
				count = count + 1
			end
			assert(count == 7)
		`},
		{"update_and_delete", `
			local t = {a = 1, b = 2, c = 3, d = 4}
			local first = next(t)
			local second = next(t, first)
			t[second] = nil
			local seen, count = {[first] = true}, 1
			local k = first
			while true do
				local v
				k, v = next(t, k)
				if k == nil then break end
				assert(not seen[k])
				seen[k] = true
				t[k] = v * 10
				count = count + 1
			end
			assert(count == 3 and not seen[second])
			count = 0
			for k, v in pairs(t) do
				assert(seen[k] and v == t[k])
				count = count + 1
			end
			assert(count == 3)
		`},
		{"insert_after_iteration", `
			local t = {a = 1, b = 2, c = 3}
			for _ in pairs(t) do end
			t.d = 4
			local seen, count = {}, 0
			for k, v in pairs(t) do
				assert(not seen[k] and t[k] == v)
				seen[k] = true
				count = count + 1
			end
			assert(count == 4 and seen.d)
		`},
		{"promote_numeric_keys", `
			local t = {[4] = 4, [8] = 8, [16] = 16, a = 17}
			for _ in pairs(t) do end
			for i = 1, 16 do t[i] = i end
			local seen, count = {}, 0
			for k, v in pairs(t) do
				assert(not seen[k] and t[k] == v)
				seen[k] = true
				count = count + 1
			end
			assert(count == 17 and seen.a)
		`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewState()
			OpenLibraries(l)
			if err := DoString(l, tt.program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkNextHash(b *testing.B) {
	for _, size := range []int{1000, 10000, 50000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			l := NewState()
			l.CreateTable(0, size)
			for i := 0; i < size; i++ {
				l.PushString(fmt.Sprintf("%d_MySuperKey", 100000000+i))
				l.PushInteger(i)
				l.SetTable(-3)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				count := 0
				for l.PushNil(); l.Next(-2); {
					count++
					l.Pop(1)
				}
				if count != size {
					b.Fatalf("visited %d entries, want %d", count, size)
				}
			}
		})
	}
}
