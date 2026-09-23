package lua

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"
)

func chunkPrefix(t *testing.T, parameterCount, maxStackSize byte) *bytes.Buffer {
	t.Helper()
	b := new(bytes.Buffer)
	if err := binary.Write(b, endianness(), header); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(b, endianness(), []int32{0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(b, endianness(), []byte{parameterCount, 1, maxStackSize}); err != nil {
		t.Fatal(err)
	}
	return b
}

func writeInts(t *testing.T, b *bytes.Buffer, values ...int32) {
	t.Helper()
	if err := binary.Write(b, endianness(), values); err != nil {
		t.Fatal(err)
	}
}

func undumpChunk(t *testing.T, b *bytes.Buffer) error {
	t.Helper()
	_, err := NewState().undump(bytes.NewReader(b.Bytes()), "crafted")
	return err
}
func TestUndumpBoundsHugeCodeCount(t *testing.T) {
	b := chunkPrefix(t, 0, 2)
	writeInts(t, b, 0x7fffffff)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	err := undumpChunk(t, b)
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("expected an error for a truncated chunk claiming 0x7fffffff instructions")
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Errorf("undump allocated %d bytes for a %d byte chunk; want under 8 MiB", allocated, b.Len())
	}
}

func TestUndumpBoundsHugeStringLength(t *testing.T) {
	b := chunkPrefix(t, 0, 2)
	writeInts(t, b, 0, 1)
	b.WriteByte(byte(TypeString))
	if err := binary.Write(b, endianness(), uint64(1<<40)); err != nil {
		t.Fatal(err)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	err := undumpChunk(t, b)
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("expected an error for a truncated chunk claiming a 1 TiB string constant")
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Errorf("undump allocated %d bytes for a %d byte chunk; want under 8 MiB", allocated, b.Len())
	}
}

func TestUndumpRejectsNegativeCounts(t *testing.T) {
	for _, test := range []struct {
		name   string
		counts []int32
	}{
		{"code", []int32{-1}},
		{"constants", []int32{0, -1}},
		{"prototypes", []int32{0, 0, -1}},
		{"upvalues", []int32{0, 0, 0, -1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := chunkPrefix(t, 0, 2)
			writeInts(t, b, test.counts...)
			if err := undumpChunk(t, b); err != errCorrupted {
				t.Errorf("expected errCorrupted for a negative %s count, got %v", test.name, err)
			}
		})
	}
}

func TestUndumpRejectsMoreUpValueNamesThanUpValues(t *testing.T) {
	b := chunkPrefix(t, 0, 2)
	writeInts(t, b, 0, 0, 0, 0)
	if err := binary.Write(b, endianness(), uint64(0)); err != nil {
		t.Fatal(err)
	}
	writeInts(t, b, 0, 0, 1)
	if err := undumpChunk(t, b); err != errCorrupted {
		t.Errorf("expected errCorrupted for 1 upvalue name against 0 upvalues, got %v", err)
	}
}

func TestUndumpRejectsDeeplyNestedPrototypes(t *testing.T) {
	b := chunkPrefix(t, 0, 2)
	for i := 0; i < maxUndumpNesting+1; i++ {
		writeInts(t, b, 0, 0, 1)
		writeInts(t, b, 0, 0)
		if err := binary.Write(b, endianness(), []byte{0, 1, 2}); err != nil {
			t.Fatal(err)
		}
	}
	if err := undumpChunk(t, b); err != errCorrupted {
		t.Errorf("expected errCorrupted for %d nested prototypes, got %v", maxUndumpNesting+1, err)
	}
}

func TestLoadReportsUndumpErrorAsSyntaxError(t *testing.T) {
	b := chunkPrefix(t, 0, 2)
	writeInts(t, b, 0x7fffffff)
	if err := LoadBuffer(NewState(), b.String(), "crafted", "b"); err != SyntaxError {
		t.Errorf("expected SyntaxError for a truncated binary chunk, got %v", err)
	}
}
