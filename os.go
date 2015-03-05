package lua

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

func field(l *State, key string, def int) int {
	l.Field(-1, key)
	r, ok := l.ToInteger(-1)
	if !ok {
		if def < 0 {
			Errorf(l, "field '%s' missing in date table", key)
		}
		r = def
	}
	l.Pop(1)
	return r
}

var osLibrary = []RegistryFunction{
	{"clock", clock},
	// {"date", os_date},
	{"difftime", func(l *State) int {
		l.PushNumber(time.Unix(int64(CheckNumber(l, 1)), 0).Sub(time.Unix(int64(OptNumber(l, 2, 0)), 0)).Seconds())
		return 1
	}},
	{"execute", func(l *State) int {
		c := OptString(l, 1, "")
		if c == "" {
			// TODO check if sh is available
			l.PushBoolean(true)
			return 1
		}
		cmd := strings.Fields(c)
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			// TODO
		}
		l.PushBoolean(true)
		l.PushString("exit")
		l.PushInteger(0)
		return 3
	}},
	{"exit", func(l *State) int {
		var status int
		if l.IsBoolean(1) {
			if !l.ToBoolean(1) {
				status = 1
			}
		} else {
			status = OptInteger(l, 1, status)
		}
		// if l.ToBoolean(2) {
		// 	Close(l)
		// }
		os.Exit(status)
		panic("unreachable")
	}},
	{"getenv", func(l *State) int { l.PushString(os.Getenv(CheckString(l, 1))); return 1 }},
	{"remove", func(l *State) int { name := CheckString(l, 1); return FileResult(l, os.Remove(name), name) }},
	{"rename", func(l *State) int { return FileResult(l, os.Rename(CheckString(l, 1), CheckString(l, 2)), "") }},
	// {"setlocale", func(l *State) int {
	// 	op := CheckOption(l, 2, "all", []string{"all", "collate", "ctype", "monetary", "numeric", "time"})
	// 	l.PushString(setlocale([]int{LC_ALL, LC_COLLATE, LC_CTYPE, LC_MONETARY, LC_NUMERIC, LC_TIME}, OptString(l, 1, "")))
	// 	return 1
	// }},
	{"time", func(l *State) int {
		if l.IsNoneOrNil(1) {
			l.PushNumber(float64(time.Now().Unix()))
		} else {
			CheckType(l, 1, TypeTable)
			l.SetTop(1)
			year := field(l, "year", -1) - 1900
			month := field(l, "month", -1) - 1
			day := field(l, "day", -1)
			hour := field(l, "hour", 12)
			min := field(l, "min", 0)
			sec := field(l, "sec", 0)
			// dst := boolField(l, "isdst") // TODO how to use dst?
			l.PushNumber(float64(time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local).Unix()))
		}
		return 1
	}},
	{"tmpname", func(l *State) int {
		f, err := ioutil.TempFile("", "lua_")
		if err != nil {
			Errorf(l, "unable to generate a unique filename")
		}
		defer f.Close()
		l.PushString(f.Name())
		return 1
	}},
}

// OSOpen opens the os library. Usually passed to Require.
func OSOpen(l *State) int {
	NewLibrary(l, osLibrary)
	return 1
}
