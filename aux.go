package lua

func MetaField(l State, index int, event string) bool {
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

func CallMeta(l State, index int, event string) bool {
	index = l.AbsIndex(index)
	if !MetaField(l, index, event) {
		return false
	}
	l.PushValue(index)
	l.Call(1, 1)
	return true
}

func ArgumentError(l State, argCount int, extraMessage string) {
	var activationRecord Debug
	if !l.Stack(0, &activationRecord) { // no stack frame?
		Error(l, "bad argument #%d (%s)", argCount, extraMessage)
		return
	}
	l.Info("n", &activationRecord)
	if activationRecord.NameKind == "method" {
		argCount--         // do not count 'self'
		if argCount == 0 { // error is in the self argument itself?
			Error(l, "calling '%s' on bad self (%s)", activationRecord.Name, extraMessage)
			return
		}
	}
	if activationRecord.Name == "" {
		if pushGlobalFunctionName(l, &activationRecord) {
			activationRecord.Name, _ = l.ToString(-1)
		} else {
			activationRecord.Name = "?"
		}
	}
	Error(l, "bad argument #%d to '%s' (%s)", argCount, activationRecord.Name, extraMessage)
}

func findField(l State, objectIndex, level int) bool {
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

func pushGlobalFunctionName(l State, activationRecord *Debug) bool {
	top := l.Top()
	l.Info("f", activationRecord) // push function
	l.PushGlobalTable()
	if findField(l, top+1, 2) {
		l.Copy(-1, top+1) // move name to proper place
		l.Pop(2)          // remove pushed values
		return true
	}
	l.SetTop(top) // remove function and global table
	return false
}

func typeError(l State, argCount int, typeName string) {
	m := l.PushFString("%s expected, got %s", typeName, TypeName(l, argCount))
	ArgumentError(l, argCount, m)
}

func tagError(l State, argCount, tag int) {
	typeError(l, argCount, l.TypeName(tag))
}

func Where(l State, level int) {
	var activationRecord Debug
	if l.Stack(level, &activationRecord) { // check function at level
		l.Info("Sl", &activationRecord)       // get info about it
		if activationRecord.CurrentLine > 0 { // is there info?
			l.PushFString("%s:%d: ", activationRecord.Source, activationRecord.CurrentLine)
			return
		}
	}
	l.PushString("") // else, no information available...
}

func Error(l State, format string, a ...interface{}) {
	Where(l, 1)
	l.PushFString(format, a...)
	l.Concat(2)
	l.Error()
}

func ToString(l State, index int) (string, bool) {
	if !CallMeta(l, index, "__tostring") {
		switch l.Type(index) {
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
			l.PushFString("%s: %p", TypeName(l, index), l.ToValue(index))
		}
	}
	return l.ToString(-1)
}

func CheckType(l State, index, t int) {
	if l.Type(index) != t {
		tagError(l, index, t)
	}
}

func CheckAny(l State, index int) {
	if l.Type(index) == TypeNone {
		ArgumentError(l, index, "value expected")
	}
}

func ArgumentCheck(l State, cond bool, index int, extraMessage string) {
	if !cond {
		ArgumentError(l, index, extraMessage)
	}
}

func CheckString(l State, index int) string {
	if s, ok := l.ToString(index); ok {
		return s
	}
	tagError(l, index, TypeString)
	panic("unreachable")
}

func OptString(l State, index int, def string) string {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckString(l, index)
}

func CheckNumber(l State, index int) float64 {
	n, ok := l.ToNumber(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return n
}

func OptNumber(l State, index int, def float64) float64 {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckNumber(l, index)
}

func CheckInteger(l State, index int) int {
	i, ok := l.ToInteger(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptInteger(l State, index, def int) int {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckInteger(l, index)
}

func CheckUnsigned(l State, index int) uint {
	i, ok := l.ToUnsigned(index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptUnsigned(l State, index int, def uint) uint {
	if l.IsNoneOrNil(index) {
		return def
	}
	return CheckUnsigned(l, index)
}

func TypeName(l State, index int) string {
	return l.TypeName(l.Type(index))
}

func SetFunctions(l State, functions []RegistryFunction, upValueCount int) {
	CheckStack(l, upValueCount, "too many upvalues")
	for _, r := range functions { // fill the table with given functions
		for i := 0; i < upValueCount; i++ { // copy upvalues to the top
			l.PushValue(-upValueCount)
		}
		l.PushGoClosure(r.Function, upValueCount) // closure with those upvalues
		l.SetField(-(upValueCount + 2), r.Name)
	}
	l.Pop(upValueCount) // remove upvalues
}

func CheckStack(l State, space int, message string) {
	// keep some extra space to run error routines, if needed
	if !l.CheckStack(space + MinStack) {
		if message != "" {
			Error(l, "stack overflow (%s)", message)
		} else {
			Error(l, "stack overflow")
		}
	}
}

func CheckOption(l State, index int, def string, list []string) int {
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

func SubTable(l State, index int, name string) bool {
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

func Require(l State, name string, f Function, global bool) {
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

func NewLibraryTable(l State, functions []RegistryFunction) {
	l.CreateTable(0, len(functions))
}

func NewLibrary(l State, functions []RegistryFunction) {
	NewLibraryTable(l, functions)
	SetFunctions(l, functions, 0)
}
