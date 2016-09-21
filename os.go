package lua

import (
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
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

	// From the Lua manual:
	// "This function is equivalent to the ISO C function system"
	// https://www.lua.org/manual/5.2/manual.html#pdf-os.execute
	{"execute", func(l *State) int {
		c := OptString(l, 1, "")

		if c == "" {
			// Check whether "sh" is available on the system.
			err := exec.Command("sh").Run()
			l.PushBoolean(err == nil)
			return 1
		}

		terminatedSuccessfully := true
		terminationReason := "exit"
		terminationData := 0

		// Create the command.
		cmd := exec.Command("sh", "-c", c)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Run the command.
		if err := cmd.Run(); err != nil {
			terminatedSuccessfully = false
			terminationReason = "exit"
			terminationData = 1

			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						terminationReason = "signal"
						terminationData = int(status.Signal())
					} else {
						terminationData = status.ExitStatus()
					}
				} else {
					// Unsupported system?
				}
			} else {
				// From man 3 system:
				// "If a child process could not be created, or its
				// status could not be retrieved, the return value
				// is -1."
				terminationData = -1
			}
		}

		// Deal with the return values.
		if terminatedSuccessfully {
			l.PushBoolean(true)
		} else {
			l.PushNil()
		}

		l.PushString(terminationReason)
		l.PushInteger(terminationData)

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
