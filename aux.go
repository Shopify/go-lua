package lua

func MetaField(l *State, index int, event string) bool {
	if !MetaTable(l, index) {
		return false
	}
	PushString(l, event)
	RawGet(l, -2)
	if IsNil(l, -1) {
		Pop(l, 2) // remove metatable and metafield
		return false
	}
	Remove(l, -2) // remove only metatable
	return true
}

func CallMeta(l *State, index int, event string) bool {
	index = AbsIndex(l, index)
	if !MetaField(l, index, event) {
		return false
	}
	PushValue(l, index)
	Call(l, 1, 1)
	return true
}

func ArgumentError(l *State, argCount int, extraMessage string) {
	var activationRecord Debug
	if !Stack(l, 0, &activationRecord) { // no stack frame?
		Errorf(l, "bad argument #%d (%s)", argCount, extraMessage)
		return
	}
	Info(l, "n", &activationRecord)
	if activationRecord.NameKind == "method" {
		argCount--         // do not count 'self'
		if argCount == 0 { // error is in the self argument itself?
			Errorf(l, "calling '%s' on bad self (%s)", activationRecord.Name, extraMessage)
			return
		}
	}
	if activationRecord.Name == "" {
		if pushGlobalFunctionName(l, &activationRecord) {
			activationRecord.Name, _ = ToString(l, -1)
		} else {
			activationRecord.Name = "?"
		}
	}
	Errorf(l, "bad argument #%d to '%s' (%s)", argCount, activationRecord.Name, extraMessage)
}

func findField(l *State, objectIndex, level int) bool {
	if level == 0 || !IsTable(l, -1) {
		return false
	}
	for PushNil(l); Next(l, -2); Pop(l, 1) { // for each pair in table
		if IsString(l, -2) { // ignore non-string keys
			if RawEqual(l, objectIndex, -1) { // found object?
				Pop(l, 1) // remove value (but keep name)
				return true
			} else if findField(l, objectIndex, level-1) { // try recursively
				Remove(l, -2) // remove table (but keep name)
				PushString(l, ".")
				Insert(l, -2) // place "." between the two names
				Concat(l, 3)
				return true
			}
		}
	}
	return false
}

func pushGlobalFunctionName(l *State, activationRecord *Debug) bool {
	top := Top(l)
	Info(l, "f", activationRecord) // push function
	PushGlobalTable(l)
	if findField(l, top+1, 2) {
		Copy(l, -1, top+1) // move name to proper place
		Pop(l, 2)          // remove pushed values
		return true
	}
	SetTop(l, top) // remove function and global table
	return false
}

func typeError(l *State, argCount int, typeName string) {
	ArgumentError(l, argCount, PushFString(l, "%s expected, got %s", typeName, TypeName(l, argCount)))
}

func tagError(l *State, argCount, tag int) { typeError(l, argCount, TypeName(l, tag)) }

func Where(l *State, level int) {
	var activationRecord Debug
	if Stack(l, level, &activationRecord) { // check function at level
		Info(l, "Sl", &activationRecord)      // get info about it
		if activationRecord.CurrentLine > 0 { // is there info?
			PushFString(l, "%s:%d: ", activationRecord.Source, activationRecord.CurrentLine)
			return
		}
	}
	PushString(l, "") // else, no information available...
}

func Errorf(l *State, format string, a ...interface{}) {
	Where(l, 1)
	PushFString(l, format, a...)
	Concat(l, 2)
	Error(l)
}

func ToStringMeta(l *State, index int) (string, bool) {
	if !CallMeta(l, index, "__tostring") {
		switch Type(l, index) {
		case TypeNumber, TypeString:
			PushValue(l, index)
		case TypeBoolean:
			if ToBoolean(l, index) {
				PushString(l, "true")
			} else {
				PushString(l, "false")
			}
		case TypeNil:
			PushString(l, "nil")
		default:
			PushFString(l, "%s: %p", TypeName(l, index), ToValue(l, index))
		}
	}
	return ToString(l, -1)
}

func CheckType(l *State, index, t int) {
	if Type(l, index) != t {
		tagError(l, index, t)
	}
}

func CheckAny(l *State, index int) {
	if Type(l, index) == TypeNone {
		ArgumentError(l, index, "value expected")
	}
}

func ArgumentCheck(l *State, cond bool, index int, extraMessage string) {
	if !cond {
		ArgumentError(l, index, extraMessage)
	}
}

func CheckString(l *State, index int) string {
	if s, ok := ToString(l, index); ok {
		return s
	}
	tagError(l, index, TypeString)
	panic("unreachable")
}

func OptString(l *State, index int, def string) string {
	if IsNoneOrNil(l, index) {
		return def
	}
	return CheckString(l, index)
}

func CheckNumber(l *State, index int) float64 {
	n, ok := ToNumber(l, index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return n
}

func OptNumber(l *State, index int, def float64) float64 {
	if IsNoneOrNil(l, index) {
		return def
	}
	return CheckNumber(l, index)
}

func CheckInteger(l *State, index int) int {
	i, ok := ToInteger(l, index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptInteger(l *State, index, def int) int {
	if IsNoneOrNil(l, index) {
		return def
	}
	return CheckInteger(l, index)
}

func CheckUnsigned(l *State, index int) uint {
	i, ok := ToUnsigned(l, index)
	if !ok {
		tagError(l, index, TypeNumber)
	}
	return i
}

func OptUnsigned(l *State, index int, def uint) uint {
	if IsNoneOrNil(l, index) {
		return def
	}
	return CheckUnsigned(l, index)
}

func TypeNameAt(l *State, index int) string {
	return TypeName(l, Type(l, index))
}

func SetFunctions(l *State, functions []RegistryFunction, upValueCount int) {
	CheckStackWithMessage(l, upValueCount, "too many upvalues")
	for _, r := range functions { // fill the table with given functions
		for i := 0; i < upValueCount; i++ { // copy upvalues to the top
			PushValue(l, -upValueCount)
		}
		PushGoClosure(l, r.Function, upValueCount) // closure with those upvalues
		SetField(l, -(upValueCount + 2), r.Name)
	}
	Pop(l, upValueCount) // remove upvalues
}

func CheckStackWithMessage(l *State, space int, message string) {
	// keep some extra space to run error routines, if needed
	if !CheckStack(l, space+MinStack) {
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
	ArgumentError(l, index, PushFString(l, "invalid option '%s'", name))
	panic("unreachable")
}

func SubTable(l *State, index int, name string) bool {
	Field(l, index, name)
	if IsTable(l, -1) {
		return true // table already there
	}
	Pop(l, 1) // remove previous result
	index = AbsIndex(l, index)
	NewTable(l)
	PushValue(l, -1)         // copy to be left at top
	SetField(l, index, name) // assign new table to field
	return false             // did not find table there
}

func Require(l *State, name string, f Function, global bool) {
	PushGoFunction(l, f)
	PushString(l, name) // argument to f
	Call(l, 1, 1)       // open module
	SubTable(l, RegistryIndex, "_LOADED")
	PushValue(l, -2)      // make copy of module (call result)
	SetField(l, -2, name) // _LOADED[name] = module
	Pop(l, 1)             // remove _LOADED table
	if global {
		PushValue(l, -1)   // copy of module
		SetGlobal(l, name) // _G[name] = module
	}
}

func NewLibraryTable(l *State, functions []RegistryFunction) {
	CreateTable(l, 0, len(functions))
}

func NewLibrary(l *State, functions []RegistryFunction) {
	NewLibraryTable(l, functions)
	SetFunctions(l, functions, 0)
}