# go-lua: A Lua 5.2 VM in Pure Go
### Francis Bogsanyi
#### Production Engineering - Shopify

---

# What is go-lua?

---

# What is ~~go-~~Lua?

- Embeddable scripting language
  - http://www.lua.org/
  - Reference implementation around 20 kLOC of C
- Used extensively in:
  - **Games**: describe levels, data, game logic & AI
  - **Nginx**: load balancing, SSL cert. management, routing, etc.
  - **Redis**: "stored procedures", inventory/reservation system

^ I assume some knowledge of Go. Alexandre will introduce the language after this talk, if you're unfamiliar. Let's focus on Lua.

^ Reference implementation is one of the fastest scripting language interpreters. `luajit` is even faster.

---

# What is go-lua?

- Several bindings to C Lua already exist for Go
- go-lua is a manual port of C Lua to Go

^ If you're familiar with the work to port Go's compiler & runtime from C to Go, you'll know Robert Griesemer implemented a pretty useful translation tool. go-lua preceded that. It was ported by hand.

---

![](brahma.jpg)

# Creation myth

- Conan load generator
  - Written & scripted in Ruby, hosted in Heroku, scaled manually
  - "**go**nan" rewrite - popular for pair programming interviews
- Genghis load generator
  - Written in Go, Heroku scheduler, EC2 workers
  - Scales to 300K RPS against Shopify
  - How to deploy new flows without deploying everything?

^ We use Genghis to simulate flash sales and other nasty things, so we can test the limits of our infrastructure. Needs a way to script it, so we can run new scripts, called "flows", without redeploying everything. go-lua is that scripting engine.

^ Actual story: I wanted to learn Go - one of the main reasons I joined Shopify. An interviewer asked if I'd be OK if I never got to use Go at Shopify - inside voice screamed "No!" :-). A good way to learn a language is to implement something you already know really well. I know a lot about implementing language runtimes - I worked at IBM on their J9 Java virtual machine. Lua has a really simple, almost textbook compiler and runtime. Over beers, a colleague joked: "The Lua VM is 20 kLOC - you should be able to port that in a weekend". It took a little longer than that :-). I'd just come off an intense project to shard Shopify and needed to decompress, so I had the perfect opportunity. Genghis was developed afterwards :-).

---

![](brahma.jpg)

# Creation myth

![inline](IMG_0048.JPG)

^ This is the high-level architecture of Genghis. I've included this here to provide context for the rest of the talk. I hope someone else from our team can go into some detail on this at a later meetup.

---

# Goals

- Low overhead Go ↔︎ Lua cross calls
- Minimal runtime data, scale to 10,000s of VM instances
- Not insanely slow
- Familiar API for C Lua developers

^ Existing C Lua bindings in Go have 2 or more managed heaps - Go's + N Lua heaps. Cgo brings additional overhead, especially if we're calling out to C from many goroutines. Our goal was to support 10s of thousands of goroutines simultaneously, each with their own Lua VM.

---

# Genghis worker

```
  for tick() {
  	go func() {
      l := lua.NewState()
      registerLibraries(l)
      registerFunctions(l)
      lua.LoadString(l, fmt.Sprintf("require %q", path))
      l.ProtectedCall(0, 0, 0)
    }()
  }
```

^ Why do we want to support this kind of scale? It provides a simple flow programming model, where an execution of a flow is  isolated from all other executions. Here's the high-level view of what's going on in one of our EC2 workers when we run a flow. We spin up a new, isolated VM instance on every tick and that VM instance runs our Lua script to completion.

---

# Example Lua script (flow)

```
info "starting"
local resp = get(config.domain)

if not resp then
    fail("resp is nil =/")
elseif not resp.body then
    fail("body is nil =/", {code=resp.code})
end
success("flow succeeded", {code=resp.code, status=resp.status})
```

^ Here's a trivial flow. All the functions called here are part of Genghis, not go-lua, and we'll see a couple examples later.

---

# Checkout random product

```
  local helpers = require("helpers")
  local common = require("common")

  helpers.add_product(config.variants)

  helpers.do_steps({
    common.begin_checkout,
    common.input_customer_information,
    common.pick_shipping_method,
    common.pay,
    common.poll_processing_page
  })
```

^  Realistic flows are more complex, and we have both generic and Shopify-specific helpers to make it easier to write flows.

---

# Implementation

- Go data types: `string`, `float64`, `nil`, `bool`, `interface{}`
- Go GC
- 8 kLOC + tests
- Custom `table` type
- Separate data and control stacks
  - Data: `[]interface{}`
  - Control: doubly-linked list of `callInfo` structs

^ The implementation is fairly straightforward: we use Go's basic data types as much as possible, so Lua strings are Go strings, Lua numbers are float64s, nil is nil, bool is bool. Lua values are represented by the empty interface type. We use Go's heap & GC rather than reimplementing that functionality. This does mean we don't support Lua's "weak tables".

^ The VM, compiler & standard libraries total around 8 thousand lines of code.

^ Lua tables act as both maps and arrays, grow dynamically, support metatables (with a negative cache) for method lookup. We needed a custom `table` data type due to the complexity of table iteration, indexing & growth in Lua.

^ We mostly mimicked C Lua's separate data and control stacks. Data stacks in C Lua and go-lua are remarkably similar: C Lua uses a pair of type + value for a stack slot, whereas go-lua uses an empty interface ... which is a type + value pair. The control stack has a record for each stack frame that describes range of data stack slots the frame occupies, the current set of active registers, the instructions for the function we're evaluating and the index of the current instruction. C Lua stores this in an array, whereas we use a doubly-linked list. I'll discuss this more later.

---

# What does it look like?

```
	l := lua.NewState()   // new VM instance

	lua.OpenLibraries(l)  // register standard libraries
	goluago.Open(l)       // expose some Go API
	luagoquery.Open(l)    // jQuery-like selectors

	setConfigTable(l, config.Config)
	registerHTTPFunctions(ctx, l, config)
	registerStatsdFunctions(l, config.Datadog)
	registerSleep(ctx, l)
	registerLoggers(l, config.Logger, config.Datadog)
	registerSearcher(l, config.FileSystem)

	return loadAndExecute(l, path)
```

^ This is an expanded version of the flow execution code I showed earlier. Note the basic steps here: create a new VM instance, register the libraries and globals we need. The `goluago` and `luagoquery` libraries are provided by separate Go packages. Both have been open-sourced.

---

# What does it look like?

```
    func registerSleep(ctx context.Context, l *lua.State) {
    	l.Register("sleep", func(l *lua.State) int {
    		ms := lua.CheckNumber(l, 1)
    		ctx, _ := context.WithTimeout(ctx, time.Duration(ms))
    		<-ctx.Done()
    		return 0
    	})
    }
```

^ Here's a `sleep` function that we've exposed to Lua from Go. It's registered as a global. It takes a VM instance and returns a count of the values returned. Here we expect a single numeric argument & we return nothing. The go-lua API provides core functions as methods on the VM instance, and useful helpers as top-level functions in the `lua` package. Here we can see `Register` is a core function, so we're calling it as a method, whereas argument type checking is built on top of API primitives, and is exposed as `lua.CheckNumber`.

---

# What does it look like?

```
    func statsdCount(l *lua.State, d DatadogClient) lua.Function {
    	return func(l *lua.State) int {
    		name := lua.CheckString(l, 1)
    		value := lua.OptInteger(l, 2, 1)
    		tags := pullTags(l, 3)
    		rate := lua.OptNumber(l, 4, 1.0)
    		if err := d.Count(name, int64(value), tags, rate); err != nil {
    			lua.Errorf(l, err.Error())
    		}
    		return 0
    	}
    }
```

^ Here's a slightly more complex example. It shows the support for optional arguments with default values as well as error handling. In go-lua, we use `panic` to implement the `Errorf` function. This is one of the least pleasant part of the API. It echos the C API faithfully, but isn't idiomatic Go.

---

# How do we use it?

- Genghis load generator workers
  - Spin up a Lua VM instance representing a user N times/sec
  - Functions exposed for HTTP, statsd, success/failure, etc.
  - Lua scripts (flows) run out of a zip archive, in memory
- Shaping generated traffic to match "real world" - *closed*
- Flow configuration data/scripts with Shopify API - *future*

---

# Shopify/goluago

- Go's API is much richer than Lua's
  - Ad-hoc exposure through, e.g. `require "goluago/time"`
- Use Go's regular expressions rather than Lua patterns
- `time.now()` in ns vs Lua's `os.clock()` in seconds
- `goluago/util` Go package for varargs, debugging, etc.

^ `goluago` is a set of packages exposing useful Go API functionality. For example, we use Go regular expressions instead of Lua patterns in `genghis`. The `util` subpackage includes useful Go functions for dumping Lua stack frames, vararg support, and recursively copying large data structures between Go & Lua.

---

# Lessons

- API design is hard
  - `panic` vs `error`, explicit vs implicit stack manipulation
- Impedance mismatch: external vs. internal `map` iteration
  - Go map iteration order random in `for ... range m {}`
  - Lua requires external iteration via `Next` function
- Go is not a terrible host for an embedded scripting language
  - ... if you don't care much about performance

^ go-lua's API changed significantly days before we opened up the project. The changes significantly improved usability. On reflection, we could have departed further from the C API and made it even more usable. A couple of key examples are the choice to use `panic` for Lua error handling, and the explicit stack frame manipulation required for accessing function arguments & return values.

^ Requires recording the iteration order when `Next` is first called, then using the recorded keys in subsequent calls.

---

# Performance

- 10x slower than C Lua
- Array sort performance worse in Go than in C Lua
  - More comparisons than necessary
  - Improved in Go 1.6 - haven't retested

---

# Performance

- Range checks for slice access

```
	  frame[i.a()] = frame[i.b()]
```

```
		                              ; ... extract i.a and i.b (omitted)
		MOVQ DI, BP
		CMPQ R10, BX                  ; range check
		JAE  0x9da02
		SHLQ $0x4, BX
		ADDQ BX, BP
		MOVQ DI, BX
		MOVQ CX, R8
		CMPQ R10, CX                  ; range check
		JAE  0x9d9fb
		SHLQ $0x4, R8
		ADDQ R8, BX
		MOVQ BX, 0x8(SP)              ; push args
		MOVQ BP, 0x10(SP)
		LEAQ 0xbe5e3(IP), BP
		MOVQ BP, 0(SP)
		CALL runtime.typedmemmove(SB) ; call runtime helper
```

^ Access to slices in Go requires range checks. Here's the implementation of the Move opcode, which simply copies one "register" to another in the Lua VM ...

^ Go compiles this with 2 range checks. On top of this, copying an element of a slice to another (or the same) slice requires a function call into the Go runtime. A C compiler would implement all of this as a load and a store.

---

# Performance

- Large, dense `switch` is a binary search

```
		switch i := ci.step(); i.opCode() {
		case opMove:
			frame[i.a()] = frame[i.b()]
		case opLoadConstant:
			frame[i.a()] = constants[i.bx()]
		case opLoadConstantEx:
			frame[i.a()] = constants[expectNext(ci, opExtraArg).ax()]
		case opLoadBool:
			frame[i.a()] = i.b() != 0
			if i.c() != 0 {
				ci.skip()
			}
		...
		}
```

^ The core bytecode interpreter is a large dense switch statement, with a case for each opcode. C compilers typically turn this into a jump table - a table where each entry is the address of the code block for one case - and an indirect branch - that is, the value we're switching on is used to index the jump table in a branch instruction.

---

# Performance

- Large, dense `switch` is a binary search

```
		SHRL $0x0, DX   ; extract opcode from instruction
		ANDL $0x3f, DX
		CMPQ $0x14, DX  ; op > 14?
		JA   0xa2186
		CMPQ $0x9, DX   ; op > 9?
		JA   0x9ea87
		CMPQ $0x4, DX   ; op > 4?
		JA   0x9dd01
		CMPQ $0x1, DX   ; op > 1?
		JA   0x9da94
		CMPQ $0x0, DX   ; op != 0?
		JNE  0x9da09
```

^ Go implements the same thing as a binary search followed by a linear search when fewer than 4 cases remain. In go-lua, this means 2-5 compare & branch pairs for instruction dispatch.

---

# Performance

- Dynamic allocation is slow for stack frames
  - Control frames are a doubly-linked list of `callInfo` structs
  - A misguided attempt to optimize memory footprint
- Alternatives:
  - Slice of `callInfo` containing both Go & Lua frame info
  - Interleave control & data stack

^ C Lua uses a C `union` to support native callout and Lua stack frames in the same `struct`

^ Go doesn't provide `union` data types, so a series of refactors left us with dynamic allocation of both the common and variant pieces of a control stack frame. We cache stack frames, so if you're calling a series of Lua functions XOR a series of Go functions, then you don't pay for allocation, but the moment you switch from a Lua callee to a Go callee, for example, you blow away the cache.

^ That said, we'd be much better off eating the memory overhead and storing control stack frames in a slice or interleaving control and data stacks.

---

# Performance

![inline](Genghis.png)

^ Minimal VM instance size is 1696 bytes on x86-64. Actually parsing & executing a trivial script requires 8840 bytes.

^ Genghis workers start over 300 goroutines per second on a c3.4xlarge instance, each with its own Lua VM that initializes, parses & executes the flow

^ Bottleneck is usually network IO, not CPU and certainly not memory

---

# Current status

- Actively used in `Shopify/genghis`
- 28 forks, 2 ahead of `Shopify/go-lua`
- Not actively developed right now
  - `#patcheswelcome` :smile:
- For everything else, there's `Shopify/goluago`

^ It's central to our genghis load generator. Other projects might use it, but I don't know about them.

^ *someone* is actively using it outside of Shopify

^ go-lua works well for our use case. It has known bugs, but our workarounds actually led to cleaner code, so ... yay bugs?
We'd like to move up to Lua 5.3, add missing library functions and improve performance, but none of those things is a priority for us & we haven't felt any pain from them.
