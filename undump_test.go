package lua

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
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
	// TODO this is brittle & should be replaced when we have a working compiler
	file, err := os.Open("checktable.bin")
	if err != nil {
		t.Fatal("couldn't open checktable.bin")
	}
	l := NewState().(*state)
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
	validate("@checktable.lua", p.source, "as source file name", t)
	validate(23, len(p.code), "instructions", t)
	validate(8, len(p.constants), "constants", t)
	validate(4, len(p.prototypes), "prototypes", t)
	validate(1, len(p.upValues), "upvalues", t)
	validate(0, len(p.localVariables), "local variables", t)
	validate(0, int(p.parameterCount), "parameters", t)
	validate(4, int(p.maxStackSize), "stack slots", t)
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
	l := NewState().(*state)
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
