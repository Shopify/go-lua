package lua

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func functionName(l *State, d Debug) string {
	switch {
	case d.NameKind != "":
		return fmt.Sprintf("function '%s'", d.Name)
	case d.What == "main":
		return "main chunk"
	case d.What == "Go":
		if pushGlobalFunctionName(l, d.callInfo) {
			s, _ := l.ToString(-1)
			l.Pop(1)
			return fmt.Sprintf("function '%s'", s)
		}
		return "?"
	}
	return fmt.Sprintf("function <%s:%d>", d.ShortSource, d.LineDefined)
}

func countLevels(l *State) int {
	li, le := 1, 1
	for _, ok := Stack(l, le); ok; _, ok = Stack(l, le) {
		li = le
		le *= 2
	}
	for li < le {
		m := (li + le) / 2
		if _, ok := Stack(l, m); ok {
			li = m + 1
		} else {
			le = m
		}
	}
	return le - 1
}

// Traceback creates and pushes a traceback of the stack l1. If message is not
// nil it is appended at the beginning of the traceback. The level parameter
// tells at which level to start the traceback.
func Traceback(l, l1 *State, message string, level int) {
	const levels1, levels2 = 12, 10
	levels := countLevels(l1)
	mark := 0
	if levels > levels1+levels2 {
		mark = levels1
	}
	buf := message
	if buf != "" {
		buf += "\n"
	}
	buf += "stack traceback:"
	for f, ok := Stack(l1, level); ok; f, ok = Stack(l1, level) {
		if level++; level == mark {
			buf += "\n\t..."
			level = levels - levels2
		} else {
			d, _ := Info(l1, "Slnt", f)
			buf += "\n\t" + d.ShortSource + ":"
			if d.CurrentLine > 0 {
				buf += fmt.Sprintf("%d:", d.CurrentLine)
			}
			buf += " in " + functionName(l, d)
			if d.IsTailCall {
				buf += "\n\t(...tail calls...)"
			}
		}
	}
	l.PushString(buf)
}

// MetaField pushes onto the stack the field event from the metatable of the
// object at index. If the object does not have a metatable, or if the
// metatable does not have this field, returns false and pushes nothing.
func MetaField(l *State, index int, event string) bool {
	if !l.MetaTable(index) {
		return false
	}
	l.PushString(event)
	l.RawGet(-2)
	if l.IsNil(-1) {
		l.Pop(2) // remove metatable and metafield
		return false
	}
	l.Remove(-2) // remove only metatable
	return true
}

// CallMeta calls a metamethod.
//
// If the object at index has a metatable and this metatable has a field event,
// this function calls this field passing the object as its only argument. In
// this case this function returns true and pushes onto the stack the value
// returned by the call. If there is no metatable or no metamethod, this
// function returns false (without pushing any value on the stack).
func CallMeta(l *State, index int, event string) bool {
	index = l.AbsIndex(index)
	if !MetaField(l, index, event) {
		return false
	}
	l.PushValue(index)
	l.Call(1, 1)
	return true
}

// ArgumentError raises an error with a standard message that includes extraMessage as a comment.
//
// This function never returns. It is an idiom to use it in Go functions as
//  lua.ArgumentError(l, args, "message")
//  panic("unreachable")
func ArgumentError(l *State, argCount int, extraMessage string) {
	f, ok := Stack(l, 0)
	if !ok { // no stack frame?
		Errorf(l, "bad argument #%d (%s)", argCount, extraMessage)
		return
	}
	d, _ := Info(l, "n", f)
	if d.NameKind == "method" {
		argCount--         // do not count 'self'
		if argCount == 0 { // error is in the self argument itself?
			Errorf(l, "calling '%s' on bad self (%s)", d.Name, extraMessage)
			return
		}
	}
	if d.Name == "" {
		if pushGlobalFunctionName(l, f) {
			d.Name, _ = l.ToString(-1)
		} else {
			d.Name = "?"
		}
	}
	Errorf(l, "bad argument #%d to '%s' (%s)", argCount, d.Name, extraMessage)
}

func findField(l *State, objectIndex, level int) bool {
	if level == 0 || !l.IsTable(-1) {
		return false
	}
	for l.PushNil(); l.Next(-2); l.Pop(1) { // for each pair in table
		if l.IsString(-2) { // ignore non-string keys
			if l.RawEqual(objectIndex, -1) { // found object?
				l.Pop(1) // remove value (but keep name)
				return true
			} else if findField(l, objectIndex, level-1) { // try recursively
				l.Remove(-2) // remove table (but keep name)
				l.PushString(".")
				l.Insert(-2) // place "." between the two names
				l.Concat(3)
				return true
			}
		}
	}
	return false
}

func pushGlobalFunctionName(l *State, f Frame) bool {
	top := l.Top()
	Info(l, "f", f) // push function
	l.PushGlobalTable()
	if findField(l, top+1, 2) {
		l.Copy(-1, top+1) // move name to proper place
		l.Pop(2)          // remove pushed values
		return true
	}
	l.SetTop(top) // remove function and global table
	return false
}

func typeError(l *State, argCount int, typeName string) {
	ArgumentError(l, argCount, l.PushString(typeName+" expected, got "+TypeNameOf(l, argCount)))
}

func tagError(l *State, argCount int, tag Type) { typeError(l, argCount, tag.String()) }

// Where pushes onto the stack a string identifying the current position of
// the control at level in the call stack. Typically this string has the
// following format:
//   chunkname:currentline:
// Level 0 is the running function, level 1 is the function that called the
// running function, etc.
//
// This function is used to build a prefix for error messages.
func Where(l *State, level int) {
	if f, ok := Stack(l, level); ok { // check function at level
		ar, _ := Info(l, "Sl", f) // get info about it
		if ar.CurrentLine > 0 {   // is there info?
			l.PushString(fmt.Sprintf("%s:%d: ", ar.ShortSource, ar.CurrentLine))
			return
		}
	}
	l.PushString("") // else, no information available...
}

// Errorf raises an error. The error message format is given by format plus
// any extra arguments, following the same rules as PushFString. It also adds
// at the beginning of the message the file name and the line number where
// the error occurred, if this information is available.
//
// This function never returns. It is an idiom to use it in Go functions as:
//   lua.Errorf(l, args)
//   panic("unreachable")
func Errorf(l *State, format string, a ...interface{}) {
	Where(l, 1)
	l.PushFString(format, a...)
	l.Concat(2)
	l.Error()
}

// ToStringMeta converts any Lua value at the given index to a Go string in a
// reasonable format. The resulting string is pushed onto the stack and also
// returned by the function.
//
// If the value has a metatable with a "__tostring" field, then ToStringMeta
// calls the corresponding metamethod with the value as argument, and uses
// the result of the call as its result.
func ToStringMeta(l *State, index int) (string, bool) {
	if !CallMeta(l, index, "__tostring") {
		switch l.TypeOf(index) {
		case TypeNumber, TypeString:
			l.PushValue(index)
		case TypeBoolean:
			if l.ToBoolean(index) {
				l.PushString("true")
			} else {
				l.PushString("false")
			}
		case TypeNil:
			l.PushString("nil")
		default:
			l.PushFString("%s: %p", TypeNameOf(l, index), l.ToValue(index))
		}
	}
	return l.ToString(-1)
}

// NewMetaTable returns false if the registry already has the key name. Otherwise,
// creates a new table to be used as a metatable for userdata, adds it to the
// registry with key name, and returns true.
//
// In both cases it pushes onto the stack the final value associated with name in
// the registry.
func NewMetaTable(l *State, name string) bool {
	if MetaTableNamed(l, name); !l.IsNil(-1) {
		return false
	}
	l.Pop(1)
	l.NewTable()
	l.PushValue(-1)
	l.SetField(RegistryIndex, name)
	return true
}

func MetaTableNamed(l *State, name string) {
	l.Field(RegistryIndex, name)
}

func SetMetaTableNamed(l *State, name string) {
	MetaTableNamed(l, name)
	l.SetMetaTable(-2)
}

func TestUserData(l *State, index int, name string) interface{} {
	if d := l.ToUserData(index); d != nil {
		if l.MetaTable(index) {
			if MetaTableNamed(l, name); !l.RawEqual(-1, -2) {
				d = nil
			}
			l.Pop(2)
			return d
		}
	}
	return nil
}

// CheckUserData checks whether the function argument at index is a userdata
// of the type name (see NewMetaTable) and returns the userdata (see
// ToUserData).
func CheckUserData(l *State, index int, name string) interface{} {
	if d := TestUserData(l, index, name); d != nil {
		return d
	}
	typeError(l, index, name)
	panic("unreachable")
}

// CheckType checks whether the function argument at index has type t. See Type for the encoding of types for t.
func CheckType(l *State, index int, t Type) {
	if l.TypeOf(index) != t {
		tagError(l, index, t)
	}
}

// CheckAny checks whether the function has an argument of any type (including nil) at position index.
func CheckAny(l *State, index int) {
	if l.TypeOf(index) == TypeNone {
		ArgumentError(l, index, "value expected")
	}
}

// ArgumentCheck checks whether cond is true. If not, raises an error with a standard message.
func ArgumentCheck(l *State, cond bool, index int, extraMessage string) {
	if !cond {
		ArgumentError(l, index, extraMessage)
	}
}

// CheckString checks whether the function argument at index is a string and returns this string.
//
// This function uses ToString to get its result, so all conversions and caveats of that function apply here.
func CheckString(l *State, index int) string {
	if s, ok := l.ToString(index); ok {
		return s
	}
	tagError(l, index, TypeString)
	panic("unreachable")
}

// OptString returns the string at index if it is a string. If this argument is
// absent or is nil, returns def. Otherwise, raises an error.
func OptString(l *State, index int, def string) string {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckString(l, index)
}

func CheckNumber(l *State, index int) float64 {
	n, ok := l.ToNumber(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return n
}

func OptNumber(l *State, index int, def float64) float64 {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckNumber(l, index)
}

func CheckInteger(l *State, index int) int {
	i, ok := l.ToInteger(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptInteger(l *State, index, def int) int {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckInteger(l, index)
}

func CheckUnsigned(l *State, index int) uint {
	i, ok := l.ToUnsigned(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptUnsigned(l *State, index int, def uint) uint {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckUnsigned(l, index)
}

func TypeNameOf(l *State, index int) string { return l.TypeOf(index).String() }

func SetFunctions(l *State, functions []RegistryFunction, upValueCount uint8) {
	uvCount := int(upValueCount)
	CheckStackWithMessage(l, uvCount, "too many upvalues")
	for _, r := range functions { // fill the table with given functions
		for i := 0; i < uvCount; i++ { // copy upvalues to the top
			l.PushValue(-uvCount)
		}
		l.PushGoClosure(r.Function, upValueCount) // closure with those upvalues
		l.SetField(-(uvCount + 2), r.Name)
	}
	l.Pop(uvCount) // remove upvalues
}

func CheckStackWithMessage(l *State, space int, message string) {
	// keep some extra space to run error routines, if needed
	if !l.CheckStack(space + MinStack) {
		if message != "" {
			Errorf(l, "stack overflow (%s)", message)
		} else {
			Errorf(l, "stack overflow")
		}
	}
}

func CheckOption(l *State, index int, def string, list []string) int {
	var name string
	if def == "" {
		name = OptString(l, index, def)
	} else {
		name = CheckString(l, index)
	}
	for i, s := range list {
		if name == s {
			return i
		}
	}
	ArgumentError(l, index, l.PushFString("invalid option '%s'", name))
	panic("unreachable")
}

func SubTable(l *State, index int, name string) bool {
	l.Field(index, name)
	if l.IsTable(-1) {
		return true // table already there
	}
	l.Pop(1) // remove previous result
	index = l.AbsIndex(index)
	l.NewTable()
	l.PushValue(-1)         // copy to be left at top
	l.SetField(index, name) // assign new table to field
	return false            // did not find table there
}

// Require calls function f with string name as an argument and sets the call
// result in package.loaded[name], as if that function had been called
// through require.
//
// If global is true, also stores the result into global name.
//
// Leaves a copy of that result on the stack.
func Require(l *State, name string, f Function, global bool) {
	l.PushGoFunction(f)
	l.PushString(name) // argument to f
	l.Call(1, 1)       // open module
	SubTable(l, RegistryIndex, "_LOADED")
	l.PushValue(-2)      // make copy of module (call result)
	l.SetField(-2, name) // _LOADED[name] = module
	l.Pop(1)             // remove _LOADED table
	if global {
		l.PushValue(-1)   // copy of module
		l.SetGlobal(name) // _G[name] = module
	}
}

func NewLibraryTable(l *State, functions []RegistryFunction) { l.CreateTable(0, len(functions)) }

func NewLibrary(l *State, functions []RegistryFunction) {
	NewLibraryTable(l, functions)
	SetFunctions(l, functions, 0)
}

func skipComment(r *bufio.Reader) (bool, error) {
	bom := "\xEF\xBB\xBF"
	if ba, err := r.Peek(len(bom)); err != nil && err != io.EOF {
		return false, err
	} else if string(ba) == bom {
		_, _ = r.Read(ba)
	}
	if c, _, err := r.ReadRune(); err != nil {
		if err == io.EOF {
			err = nil
		}
		return false, err
	} else if c == '#' {
		_, err = r.ReadBytes('\n')
		if err == io.EOF {
			err = nil
		}
		return true, err
	}
	return false, r.UnreadRune()
}

func LoadFile(l *State, fileName, mode string) error {
	var f *os.File
	fileNameIndex := l.Top() + 1
	fileError := func(what string) error {
		fileName, _ := l.ToString(fileNameIndex)
		l.PushFString("cannot %s %s", what, fileName[1:])
		l.Remove(fileNameIndex)
		return FileError
	}
	if fileName == "" {
		l.PushString("=stdin")
		f = os.Stdin
	} else {
		l.PushString("@" + fileName)
		var err error
		if f, err = os.Open(fileName); err != nil {
			return fileError("open")
		}
	}
	r := bufio.NewReader(f)
	if skipped, err := skipComment(r); err != nil {
		l.SetTop(fileNameIndex)
		return fileError("read")
	} else if skipped {
		r = bufio.NewReader(io.MultiReader(strings.NewReader("\n"), r))
	}
	s, _ := l.ToString(-1)
	err := l.Load(r, s, mode)
	if f != os.Stdin {
		_ = f.Close()
	}
	switch err {
	case nil, SyntaxError, MemoryError: // do nothing
	default:
		l.SetTop(fileNameIndex)
		return fileError("read")
	}
	l.Remove(fileNameIndex)
	return err
}

func LoadString(l *State, s string) error { return LoadBuffer(l, s, s, "") }

func LoadBuffer(l *State, b, name, mode string) error {
	return l.Load(strings.NewReader(b), name, mode)
}

// NewStateEx creates a new Lua state. It calls NewState and then sets a panic
// function that prints an error message to the standard error output in case
// of fatal errors.
//
// Returns the new state.
func NewStateEx() *State {
	l := NewState()
	if l != nil {
		_ = AtPanic(l, func(l *State) int {
			s, _ := l.ToString(-1)
			fmt.Fprintf(os.Stderr, "PANIC: unprotected error in call to Lua API (%s)\n", s)
			return 0
		})
	}
	return l
}

func LengthEx(l *State, index int) int {
	l.Length(index)
	if length, ok := l.ToInteger(-1); ok {
		l.Pop(1)
		return length
	}
	Errorf(l, "object length is not a number")
	panic("unreachable")
}

// FileResult produces the return values for file-related functions in the standard
// library (io.open, os.rename, file:seek, etc.).
func FileResult(l *State, err error, filename string) int {
	if err == nil {
		l.PushBoolean(true)
		return 1
	}
	l.PushNil()
	if filename != "" {
		l.PushString(filename + ": " + err.Error())
	} else {
		l.PushString(err.Error())
	}
	l.PushInteger(0) // TODO map err to errno
	return 3
}

// DoFile loads and runs the given file.
func DoFile(l *State, fileName string) error {
	if err := LoadFile(l, fileName, ""); err != nil {
		return err
	}
	return l.ProtectedCall(0, MultipleReturns, 0)
}

// DoString loads and runs the given string.
func DoString(l *State, s string) error {
	if err := LoadString(l, s); err != nil {
		return err
	}
	return l.ProtectedCall(0, MultipleReturns, 0)
}
