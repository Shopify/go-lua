package lua

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findLoader(l *State, name string) {
	var msg string
	if Field(l, UpValueIndex(1), "searchers"); !IsTable(l, 3) {
		Errorf(l, "'package.searchers' must be a table")
	}
	for i := 1; ; i++ {
		if RawGetInt(l, 3, i); IsNil(l, -1) {
			Pop(l, 1)
			PushString(l, msg)
			Errorf(l, "module '%s' not found: %s", name, msg)
		}
		PushString(l, name)
		if Call(l, 1, 2); IsFunction(l, -2) {
			return
		} else if IsString(l, -2) {
			msg += CheckString(l, -2)
		}
		Pop(l, 2)
	}
}

func findFile(l *State, name, field, dirSep string) (string, error) {
	Field(l, UpValueIndex(1), field)
	path, ok := ToString(l, -1)
	if !ok {
		Errorf(l, "'package.%s' must be a string", field)
	}
	return searchPath(l, name, path, ".", dirSep)
}

func checkLoad(l *State, loaded bool, fileName string) int {
	if loaded { // Module loaded successfully?
		PushString(l, fileName) // Second argument to module.
		return 2                // Return open function & file name.
	}
	m := CheckString(l, 1)
	e := CheckString(l, -1)
	Errorf(l, "error loading module '%s' from file '%s':\n\t%s", m, fileName, e)
	panic("unreachable")
}

func searcherLua(l *State) int {
	name := CheckString(l, 1)
	filename, err := findFile(l, name, "path", string(filepath.Separator))
	if err != nil {
		return 1 // Module not found in this path.
	}
	return checkLoad(l, LoadFile(l, filename, "") == nil, filename)
}

func searcherPreload(l *State) int {
	name := CheckString(l, 1)
	Field(l, RegistryIndex, "_PRELOAD")
	Field(l, -1, name)
	if IsNil(l, -1) {
		PushString(l, fmt.Sprintf("\n\tno field package.preload['%s']", name))
	}
	return 1
}

func createSearchersTable(l *State) {
	searchers := []Function{searcherPreload, searcherLua}
	CreateTable(l, len(searchers), 0)
	for i, s := range searchers {
		PushValue(l, -2)
		PushGoClosure(l, s, 1)
		RawSetInt(l, -2, i+1)
	}
}

func readable(filename string) bool {
	f, err := os.Open(filename)
	if f != nil {
		f.Close()
	}
	return err == nil
}

func searchPath(l *State, name, path, sep, dirSep string) (string, error) {
	var msg string
	if sep != "" {
		name = strings.Replace(name, sep, dirSep, -1) // Replace sep by dirSep.
	}
	path = strings.Replace(path, string(pathListSeparator), string(filepath.ListSeparator), -1)
	for _, template := range filepath.SplitList(path) {
		if template != "" {
			filename := strings.Replace(template, "?", name, -1)
			if readable(filename) {
				return filename, nil
			}
			msg = fmt.Sprintf("%s\n\tno file '%s'", msg, filename)
		}
	}
	return "", errors.New(msg)
}

func noEnv(l *State) bool {
	Field(l, RegistryIndex, "LUA_NOENV")
	b := ToBoolean(l, -1)
	Pop(l, 1)
	return b
}

func setPath(l *State, field, env, def string) {
	if path := os.Getenv(env); path == "" || noEnv(l) {
		PushString(l, def)
	} else {
		o := fmt.Sprintf("%c%c", pathListSeparator, pathListSeparator)
		n := fmt.Sprintf("%c%s%c", pathListSeparator, def, pathListSeparator)
		path = strings.Replace(path, o, n, -1)
		PushString(l, path)
	}
	SetField(l, -2, field)
}

var packageLibrary = []RegistryFunction{
	{"loadlib", func(l *State) int {
		_ = CheckString(l, 1) // path
		_ = CheckString(l, 2) // init
		PushNil(l)
		PushString(l, "dynamic libraries not enabled; check your Lua installation")
		PushString(l, "absent")
		return 3 // Return nil, error message, and where.
	}},
	{"searchpath", func(l *State) int {
		name := CheckString(l, 1)
		path := CheckString(l, 2)
		sep := OptString(l, 3, ".")
		dirSep := OptString(l, 4, string(filepath.Separator))
		f, err := searchPath(l, name, path, sep, dirSep)
		if err != nil {
			PushNil(l)
			PushString(l, err.Error())
			return 2
		}
		PushString(l, f)
		return 1
	}},
}

func PackageOpen(l *State) int {
	NewLibrary(l, packageLibrary)
	createSearchersTable(l)
	SetField(l, -2, "searchers")
	setPath(l, "path", "LUA_PATH", defaultPath)
	PushString(l, fmt.Sprintf("%c\n%c\n?\n!\n-\n", filepath.Separator, pathListSeparator))
	SetField(l, -2, "config")
	SubTable(l, RegistryIndex, "_LOADED")
	SetField(l, -2, "loaded")
	SubTable(l, RegistryIndex, "_PRELOAD")
	SetField(l, -2, "preload")
	PushGlobalTable(l)
	PushValue(l, -2)
	SetFunctions(l, []RegistryFunction{{"require", func(l *State) int {
		name := CheckString(l, 1)
		SetTop(l, 1)
		Field(l, RegistryIndex, "_LOADED")
		Field(l, 2, name)
		if ToBoolean(l, -1) {
			return 1
		}
		Pop(l, 1)
		findLoader(l, name)
		PushString(l, name)
		Insert(l, -2)
		Call(l, 2, 1)
		if !IsNil(l, -1) {
			SetField(l, 2, name)
		}
		Field(l, 2, name)
		if IsNil(l, -1) {
			PushBoolean(l, true)
			PushValue(l, -1)
			SetField(l, 2, name)
		}
		return 1
	}}}, 1)
	Pop(l, 1)
	return 1
}
