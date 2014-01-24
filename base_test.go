package lua

import "testing"

func TestBase(t *testing.T) { Call(sourceTest(t, "base.lua", RegistryFunction{"_G", BaseOpen}), 0, 0) }
