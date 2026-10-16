package lua

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
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

func setfield(l *State, key string, value int) {
	l.PushInteger(value)
	l.SetField(-2, key)
}

func setboolfield(l *State, key string, value bool) {
	l.PushBoolean(value)
	l.SetField(-2, key)
}

func setallfields(l *State, stm time.Time) {
	setfield(l, "sec", stm.Second())
	setfield(l, "min", stm.Minute())
	setfield(l, "hour", stm.Hour())
	setfield(l, "day", stm.Day())
	setfield(l, "month", int(stm.Month()))
	setfield(l, "year", stm.Year())
	setfield(l, "wday", int(stm.Weekday())+1)
	setfield(l, "yday", stm.YearDay())
	//setboolfield(l, "isdst", false) // FIXME can't found daylight saving in golang
}

var osLibrary = []RegistryFunction{
	{"clock", clock},
	{"date", func(l *State) int { //os_date
		timestamp := OptInteger(l, 2, int(time.Now().Unix()))
		now := time.Unix(int64(timestamp), 0)
		var stm time.Time
		c := OptString(l, 1, "%c")
		if c[0] == '!' { /*  UTC? */
			stm = now.UTC()
			c = c[1:]
		} else {
			stm = now.Local()
		}
		if c == "*t" {
			l.CreateTable(0, 9)
			setallfields(l, stm)
			return 1
		} else {
			strftime := func(t *time.Time, f string) string {
				var result []string
				format := []rune(f)
				add := func(str string) {
					result = append(result, str)
				}
				weekNumber := func(t *time.Time, char int) int {
					weekday := int(t.Weekday())
					if char == 'W' {
						// Monday as the first day of the week
						if weekday == 0 {
							weekday = 6
						} else {
							weekday -= 1
						}
					}
					return (t.YearDay() + 6 - weekday) / 7
				}
				for i := 0; i < len(format); i++ {
					switch format[i] {
					case '%':
						if i < len(format)-1 {
							switch format[i+1] {
							case 'a':
								add(string(t.Weekday())[:3])
							case 'A':
								add(string(t.Weekday()))
							case 'w':
								add(fmt.Sprintf("%d", t.Weekday()))
							case 'd':
								add(fmt.Sprintf("%02d", t.Day()))
							case 'b':
								add(string(t.Month())[:3])
							case 'B':
								add(string(t.Month()))
							case 'm':
								add(fmt.Sprintf("%02d", t.Month()))
							case 'y':
								add(fmt.Sprintf("%02d", t.Year()%100))
							case 'Y':
								add(fmt.Sprintf("%02d", t.Year()))
							case 'H':
								add(fmt.Sprintf("%02d", t.Hour()))
							case 'I':
								if t.Hour() == 0 {
									add(fmt.Sprintf("%02d", 12))
								} else if t.Hour() > 12 {
									add(fmt.Sprintf("%02d", t.Hour()-12))
								} else {
									add(fmt.Sprintf("%02d", t.Hour()))
								}
							case 'p':
								if t.Hour() < 12 {
									add("AM")
								} else {
									add("PM")
								}
							case 'M':
								add(fmt.Sprintf("%02d", t.Minute()))
							case 'S':
								add(fmt.Sprintf("%02d", t.Second()))
							case 'f':
								add(fmt.Sprintf("%06d", t.Nanosecond()/1000))
							case 'z':
								add(t.Format("-0700"))
							case 'Z':
								add(t.Format("MST"))
							case 'j':
								add(fmt.Sprintf("%03d", t.YearDay()))
							case 'U':
								add(fmt.Sprintf("%02d", weekNumber(t, 'U')))
							case 'W':
								add(fmt.Sprintf("%02d", weekNumber(t, 'W')))
							case 'c':
								add(t.Format("Mon Jan 2 15:04:05 2006"))
							case 'x':
								add(fmt.Sprintf("%02d/%02d/%02d", t.Month(), t.Day(), t.Year()%100))
							case 'X':
								add(fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second()))
							case '%':
								add("%")
							}
							i += 1
						}
					default:
						add(string(format[i]))
					}
				}
				return strings.Join(result, "")
			}
			result := strftime(&stm, c)
			l.PushString(result)
			return 1
		}
		return 0
	}},
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
