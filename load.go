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
	if l.Field(UpValueIndex(1), "searchers"); !l.IsTable(3) {
		Errorf(l, "'package.searchers' must be a table")
	}
	for i := 1; ; i++ {
		if l.RawGetInt(3, i); l.IsNil(-1) {
			l.Pop(1)
			l.PushString(msg)
			Errorf(l, "module '%s' not found: %s", name, msg)
		}
		l.PushString(name)
		if l.Call(1, 2); l.IsFunction(-2) {
			return
		} else if l.IsString(-2) {
			msg += CheckString(l, -2)
		}
		l.Pop(2)
	}
}

func findFile(l *State, name, field, dirSep string) (string, error) {
	l.Field(UpValueIndex(1), field)
	path, ok := l.ToString(-1)
	if !ok {
		Errorf(l, "'package.%s' must be a string", field)
	}
	return searchPath(l, name, path, ".", dirSep)
}

func checkLoad(l *State, loaded bool, fileName string) int {
	if loaded { // Module loaded successfully?
		l.PushString(fileName) // Second argument to module.
		return 2               // Return open function & file name.
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
	l.Field(RegistryIndex, "_PRELOAD")
	l.Field(-1, name)
	if l.IsNil(-1) {
		l.PushString(fmt.Sprintf("\n\tno field package.preload['%s']", name))
	}
	return 1
}

func createSearchersTable(l *State) {
	searchers := []Function{searcherPreload, searcherLua}
	l.CreateTable(len(searchers), 0)
	for i, s := range searchers {
		l.PushValue(-2)
		l.PushGoClosure(s, 1)
		l.RawSetInt(-2, i+1)
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
	l.Field(RegistryIndex, "LUA_NOENV")
	b := l.ToBoolean(-1)
	l.Pop(1)
	return b
}

func setPath(l *State, field, env, def string) {
	if path := os.Getenv(env); path == "" || noEnv(l) {
		l.PushString(def)
	} else {
		o := fmt.Sprintf("%c%c", pathListSeparator, pathListSeparator)
		n := fmt.Sprintf("%c%s%c", pathListSeparator, def, pathListSeparator)
		path = strings.Replace(path, o, n, -1)
		l.PushString(path)
	}
	l.SetField(-2, field)
}

var packageLibrary = []RegistryFunction{
	{"loadlib", func(l *State) int {
		_ = CheckString(l, 1) // path
		_ = CheckString(l, 2) // init
		l.PushNil()
		l.PushString("dynamic libraries not enabled; check your Lua installation")
		l.PushString("absent")
		return 3 // Return nil, error message, and where.
	}},
	{"searchpath", func(l *State) int {
		name := CheckString(l, 1)
		path := CheckString(l, 2)
		sep := OptString(l, 3, ".")
		dirSep := OptString(l, 4, string(filepath.Separator))
		f, err := searchPath(l, name, path, sep, dirSep)
		if err != nil {
			l.PushNil()
			l.PushString(err.Error())
			return 2
		}
		l.PushString(f)
		return 1
	}},
}

// PackageOpen opens the package library. Usually passed to Require.
func PackageOpen(l *State) int {
	NewLibrary(l, packageLibrary)
	createSearchersTable(l)
	l.SetField(-2, "searchers")
	setPath(l, "path", "LUA_PATH", defaultPath)
	l.PushString(fmt.Sprintf("%c\n%c\n?\n!\n-\n", filepath.Separator, pathListSeparator))
	l.SetField(-2, "config")
	SubTable(l, RegistryIndex, "_LOADED")
	l.SetField(-2, "loaded")
	SubTable(l, RegistryIndex, "_PRELOAD")
	l.SetField(-2, "preload")
	l.PushGlobalTable()
	l.PushValue(-2)
	SetFunctions(l, []RegistryFunction{{"require", func(l *State) int {
		name := CheckString(l, 1)
		l.SetTop(1)
		l.Field(RegistryIndex, "_LOADED")
		l.Field(2, name)
		if l.ToBoolean(-1) {
			return 1
		}
		l.Pop(1)
		findLoader(l, name)
		l.PushString(name)
		l.Insert(-2)
		l.Call(2, 1)
		if !l.IsNil(-1) {
			l.SetField(2, name)
		}
		l.Field(2, name)
		if l.IsNil(-1) {
			l.PushBoolean(true)
			l.PushValue(-1)
			l.SetField(2, name)
		}
		return 1
	}}}, 1)
	l.Pop(1)
	return 1
}
