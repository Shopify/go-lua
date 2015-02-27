package lua

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

const fileHandle = "FILE*"
const input = "_IO_input"
const output = "_IO_output"

type stream struct {
	f     *os.File
	close Function
}

func toStream(l *State) *stream { return CheckUserData(l, 1, fileHandle).(*stream) }

func toFile(l *State) *os.File {
	s := toStream(l)
	if s.close == nil {
		Errorf(l, "attempt to use a closed file")
	}
	l.assert(s.f != nil)
	return s.f
}

func newStream(l *State, f *os.File, close Function) *stream {
	s := &stream{f: f, close: close}
	l.PushUserData(s)
	SetMetaTableNamed(l, fileHandle)
	return s
}

func newFile(l *State) *stream {
	return newStream(l, nil, func(l *State) int { return FileResult(l, toStream(l).f.Close(), "") })
}

func ioFile(l *State, name string) *os.File {
	l.Field(RegistryIndex, name)
	s := l.ToUserData(-1).(*stream)
	if s.close == nil {
		Errorf(l, fmt.Sprintf("standard %s file is closed", name[len("_IO_"):]))
	}
	return s.f
}

func forceOpen(l *State, name, mode string) {
	s := newFile(l)
	flags, err := flags(mode)
	if err == nil {
		s.f, err = os.OpenFile(name, flags, 0666)
	}
	if err != nil {
		Errorf(l, fmt.Sprintf("cannot open file '%s' (%s)", name, err.Error()))
	}
}

func ioFileHelper(name, mode string) Function {
	return func(l *State) int {
		if !l.IsNoneOrNil(1) {
			if name, ok := l.ToString(1); ok {
				forceOpen(l, name, mode)
			} else {
				toFile(l)
				l.PushValue(1)
			}
			l.SetField(RegistryIndex, name)
		}
		l.Field(RegistryIndex, name)
		return 1
	}
}

func closeHelper(l *State) int {
	s := toStream(l)
	close := s.close
	s.close = nil
	return close(l)
}

func close(l *State) int {
	if l.IsNone(1) {
		l.Field(RegistryIndex, output)
	}
	toFile(l)
	return closeHelper(l)
}

func write(l *State, f *os.File, argIndex int) int {
	var err error
	for argCount := l.Top(); argIndex < argCount && err == nil; argIndex++ {
		if n, ok := l.ToNumber(argIndex); ok {
			_, err = f.WriteString(numberToString(n))
		} else {
			_, err = f.WriteString(CheckString(l, argIndex))
		}
	}
	if err == nil {
		return 1
	}
	return FileResult(l, err, "")
}

func readNumber(l *State, f *os.File) (err error) {
	var n float64
	if _, err = fmt.Fscanf(f, "%f", &n); err == nil {
		l.PushNumber(n)
	} else {
		l.PushNil()
	}
	return
}

func read(l *State, f *os.File, argIndex int) int {
	resultCount := 0
	var err error
	if argCount := l.Top() - 1; argCount == 0 {
		//		err = readLineHelper(l, f, true)
		resultCount = argIndex + 1
	} else {
		// TODO
	}
	if err != nil {
		return FileResult(l, err, "")
	}
	if err == io.EOF {
		l.Pop(1)
		l.PushNil()
	}
	return resultCount - argIndex
}

func readLine(l *State) int {
	s := l.ToUserData(UpValueIndex(1)).(*stream)
	argCount, _ := l.ToInteger(UpValueIndex(2))
	if s.close == nil {
		Errorf(l, "file is already closed")
	}
	l.SetTop(1)
	for i := 1; i <= argCount; i++ {
		l.PushValue(UpValueIndex(3 + i))
	}
	resultCount := read(l, s.f, 2)
	l.assert(resultCount > 0)
	if !l.IsNil(-resultCount) {
		return resultCount
	}
	if resultCount > 1 {
		m, _ := l.ToString(-resultCount + 1)
		Errorf(l, m)
	}
	if l.ToBoolean(UpValueIndex(3)) {
		l.SetTop(0)
		l.PushValue(UpValueIndex(1))
		closeHelper(l)
	}
	return 0
}

func lines(l *State, shouldClose bool) {
	argCount := l.Top() - 1
	ArgumentCheck(l, argCount <= MinStack-3, MinStack-3, "too many options")
	l.PushValue(1)
	l.PushInteger(argCount)
	l.PushBoolean(shouldClose)
	for i := 1; i <= argCount; i++ {
		l.PushValue(i + 1)
	}
	l.PushGoClosure(readLine, uint8(3+argCount))
}

func flags(m string) (f int, err error) {
	if len(m) > 0 && m[len(m)-1] == 'b' {
		m = m[:len(m)-1]
	}
	switch m {
	case "r":
		f = os.O_RDONLY
	case "r+":
		f = os.O_RDWR
	case "w":
		f = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "w+":
		f = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a":
		f = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	case "a+":
		f = os.O_RDWR | os.O_CREATE | os.O_APPEND
	default:
		err = os.ErrInvalid
	}
	return
}

var ioLibrary = []RegistryFunction{
	{"close", close},
	{"flush", func(l *State) int { return FileResult(l, ioFile(l, output).Sync(), "") }},
	{"input", ioFileHelper(input, "r")},
	{"lines", func(l *State) int {
		if l.IsNone(1) {
			l.PushNil()
		}
		if l.IsNil(1) { // No file name.
			l.Field(RegistryIndex, input)
			l.Replace(1)
			toFile(l)
			lines(l, false)
		} else {
			forceOpen(l, CheckString(l, 1), "r")
			l.Replace(1)
			lines(l, true)
		}
		return 1
	}},
	{"open", func(l *State) int {
		name := CheckString(l, 1)
		flags, err := flags(OptString(l, 2, "r"))
		s := newFile(l)
		ArgumentCheck(l, err == nil, 2, "invalid mode")
		s.f, err = os.OpenFile(name, flags, 0666)
		if err == nil {
			return 1
		}
		return FileResult(l, err, name)
	}},
	{"output", ioFileHelper(output, "w")},
	{"popen", func(l *State) int { Errorf(l, "'popen' not supported"); panic("unreachable") }},
	{"read", func(l *State) int { return read(l, ioFile(l, input), 1) }},
	{"tmpfile", func(l *State) int {
		s := newFile(l)
		f, err := ioutil.TempFile("", "")
		if err == nil {
			s.f = f
			return 1
		}
		return FileResult(l, err, "")
	}},
	{"type", func(l *State) int {
		CheckAny(l, 1)
		if f, ok := TestUserData(l, 1, fileHandle).(*stream); !ok {
			l.PushNil()
		} else if f.close == nil {
			l.PushString("closed file")
		} else {
			l.PushString("file")
		}
		return 1
	}},
	{"write", func(l *State) int { return write(l, ioFile(l, output), 1) }},
}

var fileHandleMethods = []RegistryFunction{
	{"close", close},
	{"flush", func(l *State) int { return FileResult(l, toFile(l).Sync(), "") }},
	{"lines", func(l *State) int { toFile(l); lines(l, false); return 1 }},
	{"read", func(l *State) int { return read(l, toFile(l), 2) }},
	{"seek", func(l *State) int {
		whence := []int{os.SEEK_SET, os.SEEK_CUR, os.SEEK_END}
		f := toFile(l)
		op := CheckOption(l, 2, "cur", []string{"set", "cur", "end"})
		p3 := OptNumber(l, 3, 0)
		offset := int64(p3)
		ArgumentCheck(l, float64(offset) == p3, 3, "not an integer in proper range")
		ret, err := f.Seek(offset, whence[op])
		if err != nil {
			return FileResult(l, err, "")
		}
		l.PushNumber(float64(ret))
		return 1
	}},
	{"setvbuf", func(l *State) int { // Files are unbuffered in Go. Fake support for now.
		//		f := toFile(l)
		//		op := CheckOption(l, 2, "", []string{"no", "full", "line"})
		//		size := OptInteger(l, 3, 1024)
		// TODO err := setvbuf(f, nil, mode[op], size)
		return FileResult(l, nil, "")
	}},
	{"write", func(l *State) int { l.PushValue(1); return write(l, toFile(l), 2) }},
	//	{"__gc", },
	{"__tostring", func(l *State) int {
		if s := toStream(l); s.close == nil {
			l.PushString("file (closed)")
		} else {
			l.PushString(fmt.Sprintf("file (%p)", s.f))
		}
		return 1
	}},
}

func dontClose(l *State) int {
	toStream(l).close = dontClose
	l.PushNil()
	l.PushString("cannot close standard file")
	return 2
}

func registerStdFile(l *State, f *os.File, reg, name string) {
	newStream(l, f, dontClose)
	if reg != "" {
		l.PushValue(-1)
		l.SetField(RegistryIndex, reg)
	}
	l.SetField(-2, name)
}

// IOOpen opens the io library. Usually passed to Require.
func IOOpen(l *State) int {
	NewLibrary(l, ioLibrary)

	NewMetaTable(l, fileHandle)
	l.PushValue(-1)
	l.SetField(-2, "__index")
	SetFunctions(l, fileHandleMethods, 0)
	l.Pop(1)

	registerStdFile(l, os.Stdin, input, "stdin")
	registerStdFile(l, os.Stdout, output, "stdout")
	registerStdFile(l, os.Stderr, "", "stderr")

	return 1
}
