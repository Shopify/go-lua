package lua

import "testing"

// TestXPCallInvokesErrorHandler checks that xpcall calls the error handler
// passed as its second argument. The handler lives in the caller's frame, so
// the VM has to remember its absolute stack position while the protected
// function runs.
func TestXPCallInvokesErrorHandler(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	// Put a non-callable value in the first stack slot. A stale frame-relative
	// error-handler index resolves to this slot, so the bug surfaces as
	// "error in error handling" instead of an endless recursion into the
	// outermost chunk.
	l.PushNumber(0)
	err := DoString(l, `
		local ok, msg = xpcall(function() error("boom") end,
			function(m) return "handled: " .. m end)
		assert(ok == false, "xpcall reported success for a failing function")
		assert(msg:sub(1, 9) == "handled: ", "handler result: " .. tostring(msg))
		assert(msg:sub(-4) == "boom", "handler result: " .. tostring(msg))
	`)
	if err != nil {
		t.Fatal(err)
	}
}

// TestXPCallReturnsResults checks that a successful xpcall still returns the
// protected function's results and true.
func TestXPCallReturnsResults(t *testing.T) {
	l := NewState()
	OpenLibraries(l)
	err := DoString(l, `
		local ok, a, b = xpcall(function() return 1, 2 end, function() return "unused" end)
		assert(ok == true, "xpcall reported failure for a successful function")
		assert(a == 1 and b == 2, "results: " .. tostring(a) .. ", " .. tostring(b))
	`)
	if err != nil {
		t.Fatal(err)
	}
}
