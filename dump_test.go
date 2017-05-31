package lua

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUndumpThenDumpReturnsTheSameFunction(t *testing.T) {
	_, err := exec.LookPath("luac")
	if err != nil {
		t.Skipf("testing dump requires luac: %s", err)
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

	var out bytes.Buffer
	err = l.Dump(&out)
	if err != nil {
		t.Error("unexpected error", err, "with testing dump")
	}

	expectedBinary, err := ioutil.ReadFile(binary)
	if err != nil {
		t.Error("error reading file", err)
	}
	actualBinary, err := ioutil.ReadAll(&out)
	if err != nil {
		t.Error("error reading out bugger", err)
	}
	if !bytes.Equal(expectedBinary, actualBinary) {
		t.Errorf("binary chunks are not the same: %v %v", expectedBinary, actualBinary)
	}
}

func TestDumpThenUndumpReturnsTheSameFunction(t *testing.T) {
	_, err := exec.LookPath("luac")
	if err != nil {
		t.Skipf("testing dump requires luac: %s", err)
	}
	source := filepath.Join("lua-tests", "checktable.lua")
	l := NewState()
	err = LoadFile(l, source, "")
	if err != nil {
		t.Error("unexpected error", err, "with loading file", source)
	}

	var out bytes.Buffer
	f := l.stack[l.top-1].(*luaClosure)
	err = l.Dump(&out)
	if err != nil {
		t.Error("unexpected error", err, "with testing dump")
	}

	closure, err := l.undump(&out, "test")
	if err != nil {
		t.Error("unexpected error", err)
	}
	if closure == nil {
		t.Fatal("closure was nil")
	}
	undumpedPrototype := closure.prototype
	if undumpedPrototype == nil {
		t.Fatal("prototype was nil")
	}

	if !reflect.DeepEqual(f.prototype, undumpedPrototype) {
		t.Errorf("prototypes not the same: %#v %#v", f.prototype, undumpedPrototype)
	}
}
