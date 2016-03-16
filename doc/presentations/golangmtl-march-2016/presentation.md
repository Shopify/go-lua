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
  - **Redis**: "stored procedures"

^ Assume some knowledge of Go, so let's focus on Lua.

---

# What is go-lua?

- Several bindings to C Lua already exist for Go
- go-lua is a manual port of C Lua to Go

---

# Origin myth

- Conan load generator
  - Written & scripted in Ruby, hosted in Heroku, scaled manually
  - "**go**nan" rewrite - popular for pair programming interviews
- Genghis load generator
  - Written in Go, Heroku scheduler, EC2 workers
  - Scales to 20M RPM against Shopify
  - How to deploy new flows without deploying everything?

^ We use Genghis to simulate flash sales and other nasty things, so we can test the limits of our infrastructure. Needs a way to script it, so we can run new scripts, called "flows", without redeploying everything. go-lua is that scripting engine.

^ Actual story: I wanted to learn Go - one of the main reasons I joined Shopify. An interviewer asked if I'd be OK if I never got to use Go at Shopify - inside voice screamed "No!" :-). A good way to learn a language is to implement something you already know really well. I know a lot about implementing language runtimes - I worked at IBM on their J9 Java virtual machine. Lua has a really simple, almost textbook compiler and runtime. Over beers, a colleague joked: "The Lua VM is 20 kLOC - you should be able to port that in a weekend". It took a little longer than that :-). I'd just come off an intense project to shard Shopify and needed to decompress, so I had the perfect opportunity. Genghis was developed afterwards :-).

---

# Goals

- Low overhead Go ↔︎ Lua cross calls
- Minimal runtime data, scale to 10,000s of VM instances
- Not insanely slow
- Familiar API for C Lua developers

^ Existing C Lua bindings in Go have 2 or more managed heaps - Go's + N Lua heaps. Cgo brings additional overhead, especially if we're calling out to C from many goroutines. Our goal was to support 10s of thousands of goroutines simultaneously, each with their own Lua VM.

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

---

# How do we use it?

---

# Lessons

- Impedance mismatch: external vs. internal `map` iteration
  - Go map iteration order random in `for ... range m {}`
  - Lua requires external iteration via `Next` function
- Go is not a terrible host for an embedded scripting language
  - ... if you don't care much about performance

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

```go
frame[i.a()] = frame[i.b()]
```

```x86asm
                              ; ... extract i.a and i.b (omitted)
MOVQ DI, BP
CMPQ R10, BX                  ; range check
JAE 0x9da02
SHLQ $0x4, BX
ADDQ BX, BP
MOVQ DI, BX
MOVQ CX, R8
CMPQ R10, CX                  ; range check
JAE 0x9d9fb
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

```x86asm
SHRL $0x0, DX   ; extract opcode from instruction
ANDL $0x3f, DX
CMPQ $0x14, DX  ; op > 14?
JA 0xa2186
CMPQ $0x9, DX   ; op > 9?
JA 0x9ea87
CMPQ $0x4, DX   ; op > 4?
JA 0x9dd01
CMPQ $0x1, DX   ; op > 1?
JA 0x9da94
CMPQ $0x0, DX   ; op != 0?
JNE 0x9da09
```

^ The core bytecode interpreter is a large dense switch statement, with a case for each opcode. C compilers typically turn this into a jump table - a table where each entry is the address of the code block for one case - and an indirect branch - that is, the value we're switching on is used to index the jump table in a branch instruction. Go implements the same thing as a binary search followed by a linear search when fewer than 4 cases remain. In go-lua, this means 2-5 compare & branch pairs for instruction dispatch.

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

> insert :dog: graphs here

---

# Current status

- Actively used in `Shopify/genghis`
- 28 forks, 2 ahead of `Shopify/go-lua`
- Not actively developed right now
- `Shopify/goluago` fills many gaps

^ It's central to our genghis load generator. Other projects might use it, but I don't know about them.

^ *someone* is actively using it outside of Shopify

^ go-lua works well for our use case. It has known bugs, but our workarounds actually led to cleaner code, so ... yay bugs?
We'd like to move up to Lua 5.3, add missing library functions and improve performance, but none of those things is a priority for us & we haven't felt any pain from them.

^ `goluago` is a set of packages exposing useful Go API functionality. For example, we use Go regular expressions instead of Lua patterns in `genghis`. Some of this is much more useful than Lua API functions, for example `time.now` has nanosecond resolution.
