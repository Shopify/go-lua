package lua

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

func field(l *State, key string, def int) int {
	Field(l, -1, key)
	r, ok := ToInteger(l, -1)
	if !ok {
		if def < 0 {
			Errorf(l, "field '%s' missing in date table", key)
		}
		r = def
	}
	Pop(l, 1)
	return r
}

var osLibrary = []RegistryFunction{
	// {"clock", func(l *State) int { PushNumber(l, clock()/clocksPerSec); return 1 }},
	// {"date", os_date},
	{"difftime", func(l *State) int {
		PushNumber(l, time.Unix(int64(CheckNumber(l, 1)), 0).Sub(time.Unix(int64(OptNumber(l, 2, 0)), 0)).Seconds())
		return 1
	}},
	{"execute", func(l *State) int {
		c := OptString(l, 1, "")
		if c == "" {
			// TODO check if sh is available
			PushBoolean(l, true)
			return 1
		}
		cmd := strings.Fields(c)
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			// TODO
		}
		PushBoolean(l, true)
		PushString(l, "exit")
		PushInteger(l, 0)
		return 3
	}},
	{"exit", func(l *State) int {
		var status int
		if IsBoolean(l, 1) {
			if !ToBoolean(l, 1) {
				status = 1
			}
		} else {
			status = OptInteger(l, 1, status)
		}
		// if ToBoolean(l, 2) {
		// 	Close(l)
		// }
		os.Exit(status)
		panic("unreachable")
	}},
	{"getenv", func(l *State) int { PushString(l, os.Getenv(CheckString(l, 1))); return 1 }},
	{"remove", func(l *State) int { name := CheckString(l, 1); return FileResult(l, os.Remove(name), name) }},
	{"rename", func(l *State) int { return FileResult(l, os.Rename(CheckString(l, 1), CheckString(l, 2)), "") }},
	// {"setlocale", func(l *State) int {
	// 	op := CheckOption(l, 2, "all", []string{"all", "collate", "ctype", "monetary", "numeric", "time"})
	// 	PushString(l, setlocale([]int{LC_ALL, LC_COLLATE, LC_CTYPE, LC_MONETARY, LC_NUMERIC, LC_TIME}, OptString(l, 1, "")))
	// 	return 1
	// }},
	{"time", func(l *State) int {
		if IsNoneOrNil(l, 1) {
			PushNumber(l, float64(time.Now().Unix()))
		} else {
			CheckType(l, 1, TypeTable)
			SetTop(l, 1)
			year := field(l, "year", -1) - 1900
			month := field(l, "month", -1) - 1
			day := field(l, "day", -1)
			hour := field(l, "hour", 12)
			min := field(l, "min", 0)
			sec := field(l, "sec", 0)
			// dst := boolField(l, "isdst") // TODO how to use dst?
			PushNumber(l, float64(time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local).Unix()))
		}
		return 1
	}},
	{"tmpname", func(l *State) int {
		f, err := ioutil.TempFile("", "lua_")
		if err != nil {
			Errorf(l, "unable to generate a unique filename")
		}
		defer f.Close()
		PushString(l, f.Name())
		return 1
	}},
}

func OSOpen(l *State) int {
	NewLibrary(l, osLibrary)
	return 1
}
