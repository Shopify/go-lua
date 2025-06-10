package lua_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/Shopify/go-lua"
	"github.com/stretchr/testify/assert"
)

func TestPushFStringPointer(t *testing.T) {
	l := lua.NewState()
	l.PushFString("%p %s", l, "test")

	expected := fmt.Sprintf("%p %s", l, "test")
	actual := lua.CheckString(l, -1)
	if expected != actual {
		t.Errorf("PushFString, expected \"%s\" but found \"%s\"", expected, actual)
	}
}

func TestToBooleanOutOfRange(t *testing.T) {
	l := lua.NewState()
	l.SetTop(0)
	l.PushBoolean(false)
	l.PushBoolean(true)

	for i, want := range []bool{false, true, false, false} {
		idx := 1 + i
		if got := l.ToBoolean(idx); got != want {
			t.Errorf("l.ToBoolean(%d) = %t; want %t", idx, got, want)
		}
	}
}

func TestInputOutputErrorRedirect(t *testing.T) {
	t.Run("test redirect stdout", func(t *testing.T) {
		// use a variable as stdout
		reader, writer, err := os.Pipe()
		assert.Nil(t, err)

		// setup runtime state
		l := lua.NewState()
		l.SetStdout(writer)
		lua.OpenLibraries(l)

		// run lua code
		err = lua.DoString(l, "print(1 + 1)")
		assert.Nil(t, err)

		writer.Close()
		output, err := io.ReadAll(reader)
		assert.Nil(t, err)

		assert.Equal(t, "2\n", string(output))
	})

	t.Run("test std redirect", func(t *testing.T) {
		// create a pipe to stdin and add some lua code it it
		inputReader, inputWriter, err := os.Pipe()
		assert.Nil(t, err)

		outputReader, outputWriter, err := os.Pipe()
		assert.Nil(t, err)

		// setup runtime state
		l := lua.NewState()
		l.SetStdin(inputReader)
		l.SetStdout(outputWriter)
		lua.OpenLibraries(l)

		// write to the file input
		_, err = inputWriter.Write([]byte("print(1 + 1)"))
		assert.Nil(t, err)
		assert.Nil(t, inputWriter.Close())

		// run lua code
		err = lua.DoFile(l, "")
		assert.Nil(t, err)
		assert.Nil(t, outputWriter.Close())

		output, err := io.ReadAll(outputReader)
		assert.Nil(t, err)

		assert.Equal(t, "2\n", string(output))
	})

}
