package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePreservesNumericStrings(t *testing.T) {
	for _, writer := range []string{"io.write", "f:write"} {
		t.Run(writer, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "output")
			l := NewState()
			OpenLibraries(l)
			l.PushString(filename)
			l.SetGlobal("filename")
			if err := DoString(l, fmt.Sprintf(`
				local f = assert(io.open(filename, "wb"))
				io.output(f)
				assert(%s("123 ", "456 ", "Hello, ", "world!\n") == f)
				assert(%s("001.20 ", " 0x10 ", "1e+02 ", 2.5, "\n") == f)
				assert(f:close())
			`, writer, writer)); err != nil {
				t.Fatal(err)
			}
			output, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			const expected = "123 456 Hello, world!\n001.20  0x10 1e+02 2.5\n"
			if string(output) != expected {
				t.Errorf("got %q, want %q", output, expected)
			}
		})
	}
}
