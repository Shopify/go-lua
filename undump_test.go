package lua

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAllHeaderNoFun(t *testing.T) {
	expectErrorFromUndump(io.EOF, header, t)
}

func TestWrongEndian(t *testing.T) {
	h := header
	if 0 == h.Endianness {
		h.Endianness = 1
	} else {
		h.Endianness = 0
	}
	expectErrorFromUndump(errIncompatible, h, t)
}

func TestWrongVersion(t *testing.T) {
	h := header
	h.Version += 1
	expectErrorFromUndump(errVersionMismatch, h, t)
}

func TestWrongNumberSize(t *testing.T) {
	h := header
	h.NumberSize /= 2
	expectErrorFromUndump(errIncompatible, h, t)
}

func TestCorruptTail(t *testing.T) {
	h := header
	h.Tail[3] += 1
	expectErrorFromUndump(errCorrupted, h, t)
}

func TestUndump(t *testing.T) {
	_, err := exec.LookPath("luac")
	if err != nil {
		t.Skipf("testing undump requires luac: %s", err)
	}
	source := filepath.Join("lua-tests", "checktable.lua")
	binary := filepath.Join("lua-tests", "checktable.bin")
	if err := exec.Command("luac", "-o", binary, source).Run(); err != nil {
		t.Fatalf("luac failed to compile %s: %s", source, err)
	}
	file, err := os.Open(binary)
	if err != nil {
		t.Fatal("couldn't open checktable.bin")
	}
	l := NewState()
	closure, err := l.undump(file, "test")
	if err != nil {
		offset, _ := file.Seek(0, 1)
		t.Error("unexpected error", err, "at file offset", offset)
	}
	if closure == nil {
		t.Error("closure was nil")
	}
	p := closure.prototype
	if p == nil {
		t.Fatal("prototype was nil")
	}
	validate("@lua-tests/checktable.lua", p.source, "as source file name", t)
	validate(23, len(p.code), "instructions", t)
	validate(8, len(p.constants), "constants", t)
	validate(4, len(p.prototypes), "prototypes", t)
	validate(1, len(p.upValues), "upvalues", t)
	validate(0, len(p.localVariables), "local variables", t)
	validate(0, p.parameterCount, "parameters", t)
	validate(4, p.maxStackSize, "stack slots", t)
	if !p.isVarArg {
		t.Error("expected main function to be var arg, but wasn't")
	}
}

func validate(expected, actual interface{}, description string, t *testing.T) {
	if expected != actual {
		t.Errorf("expected %v %s in main function but found %v", expected, description, actual)
	}
}

func expectErrorFromUndump(expected error, data interface{}, t *testing.T) {
	l := NewState()
	_, err := l.undump(readerOn(data, t), "test")
	if err != expected {
		t.Error("expected", expected, "but got", err)
	}
}

func readerOn(data interface{}, t *testing.T) io.Reader {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, endianness(), data); err != nil {
		t.Fatal("couldn't serialize data -", err)
	}
	return buf
}
