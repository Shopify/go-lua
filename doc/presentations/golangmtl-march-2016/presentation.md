# go-lua: A Lua 5.2 VM in Pure Go
### Francis Bogsanyi
### Production Engineering - Shopify

---

# What is go-lua?

- 

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
  - "**go**nan" rewrite - popular for pair programming in interviews
- Genghis load generator
  - Written in Go, Heroku scheduler, EC2 workers
  - Scales to 20M RPM against Shopify
  - How to deploy new flows without deploying everything?

^ We use Genghis to simulate flash sales and other nasty things, so we can test the limits of our infrastructure. Needs a way to script it, so we can run new scripts, called "flows", without redeploying everything. go-lua is that scripting engine.

^ Actual story: I wanted to learn Go - one of the main reasons I joined Shopify. An interviewer asked if I'd be OK if I never got to use Go at Shopify - inside voice screamed "No!" :-). A good way to learn a language is to implement something you already know really well. I know a lot about implementing language runtimes - I worked at IBM on their J9 Java virtual machine. Lua has a really simple, almost textbook compiler and runtime. Over beers, a colleague joked: "The Lua VM is 20 kLOC - you should be able to port that in a weekend". It took a little longer than that :-). I'd just come off an intense project to shard Shopify and needed to decompress, so I had the perfect opportunity. Genghis was developed afterwards :-).

---

# Goals

- Low overhead Go ↔︎ Lua cross calls
- 

---

# Implementation

- Go data types: `string`, `float64`, `nil`, `bool`, `interface{}`
- Go GC
- 8 kLOC + tests
- Custom `table` type
- Separate data and control stacks
  - Data: `[]interface{}`
  - Control: doubly-linked list of `callInfo` structs

^ Lua tables act as both maps and arrays, grow dynamically, support metatables (with a negative cache)

---

# Lessons

- Impedance mismatch: external vs. internal `map` iteration
  - Go map iteration order is randomized, via `for ... range m` loop
  - Lua requires external iteration via `Next` function

^ Requires recording the iteration order when `Next` is first called, then using the recorded keys in subsequent calls.

---

# Performance

- 10x slower than C Lua
- Array sort performance worse in Go than in C Lua
  - More comparisons than necessary
  - Improved in Go 1.6 - haven't retested
- 

---

# What does it look like?

---

# How do we use it?

---

# Current status

---

