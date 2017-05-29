// validates what you wrote you can read in successfuly

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

	out := new(bytes.Buffer)
	err = l.dump(p, out)
	if err != nil {
		t.Error("unexpected error", err, "with testing dump")
	}

	expectedContent, err := ioutil.ReadFile(source)
	if err != nil {
		t.Error("error reading file", err)
	}

	actualContent, err := ioutil.ReadAll(out)
	if err != nil {
		t.Error("error reading out bugger", err)
	}

	if bytes.Equal(expectedContent, actualContent) {
		t.Error("not the same")
	}
}

func TestDumpThenUndumpReturnsTheSameFunction(t *testing.T) {
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
	p := new(prototype)
	out := new(bytes.Buffer)

	err = l.dump(p, out)
	if err != nil {
		t.Error("unexpected error", err, "with testing dump")
	}

	closure, err := l.undump(file, "test")
	if err != nil {
		offset, _ := file.Seek(0, 1)
		t.Error("unexpected error", err, "at file offset", offset)
	}
	if closure == nil {
		t.Error("closure was nil")
	}
	undumpedPrototype := closure.prototype
	if undumpedPrototype == nil {
		t.Fatal("prototype was nil")
	}

	if reflect.DeepEqual(p, undumpedPrototype) {
		t.Error("not the same")
	}

}

//refelct deep equal to compare the structs from dump then undump
