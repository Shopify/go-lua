package lua

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

// TODO: add missing tests

func TestReadAll(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	output := captureOutput(func() {
		DoFile(l, "fixtures/read_all.lua")
	})

	expected := `A banana contains 75% water.
The most consumed fruit in America is the banana
Bananas are a good source of vitamin C, potassium and fiber.
Fresh apples float because they contain 25% air.
Bananas contain no fat, cholesterol or sodium.`

	if strings.Trim(output, "\n") != expected {
		t.Errorf("Expecting:\n%s\nbut received:\n%s\n", expected, output)
	}
}

func TestReadLines(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	output := captureOutput(func() {
		DoFile(l, "fixtures/read_lines.lua")
	})

	expected := `A banana contains 75% water.

The most consumed fruit in America is the banana`

	if strings.Trim(output, "\n") != expected {
		t.Errorf("Expecting:\n%s\nbut received:\n%s\n", expected, output)
	}
}

func TestReadNumber(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	output := captureOutput(func() {
		DoFile(l, "fixtures/read_number.lua")
	})

	expected := "12345"
	if strings.Trim(output, "\n") != expected {
		t.Errorf("Expecting:\n%s\nbut received:\n%s\n", expected, output)
	}
}

func TestReadBytes(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	output := captureOutput(func() {
		DoFile(l, "fixtures/read_bytes.lua")
	})

	expected := "A banana contains 75"
	if strings.Trim(output, "\n") != expected {
		t.Errorf("Expecting:\n%s\nbut received:\n%s\n", expected, output)
	}
}

func captureOutput(f func()) string {
	rescueStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	out, _ := ioutil.ReadAll(r)
	fmt.Println(string(out))
	os.Stdout = rescueStdout
	return string(out)
}
