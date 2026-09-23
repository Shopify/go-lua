package lua

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unsafe"
)

type loadState struct {
	in    io.Reader
	order binary.ByteOrder
	depth int
}

const (
	undumpBatchSize  = 4096
	maxUndumpNesting = 200
)

var header struct {
	Signature                            [4]byte
	Version, Format, Endianness, IntSize byte
	PointerSize, InstructionSize         byte
	NumberSize, IntegralNumber           byte
	Tail                                 [6]byte
}

var (
	errUnknownConstantType = errors.New("lua: unknown constant type in lua binary")
	errNotPrecompiledChunk = errors.New("lua: is not a precompiled chunk")
	errVersionMismatch     = errors.New("lua: version mismatch in precompiled chunk")
	errIncompatible        = errors.New("lua: incompatible precompiled chunk")
	errCorrupted           = errors.New("lua: corrupted precompiled chunk")
)

func (state *loadState) read(data interface{}) error {
	return binary.Read(state.in, state.order, data)
}

func (state *loadState) readNumber() (f float64, err error) {
	err = state.read(&f)
	return
}

func (state *loadState) readInt() (i int32, err error) {
	err = state.read(&i)
	return
}

func (state *loadState) readPC() (pc, error) {
	i, err := state.readInt()
	return pc(i), err
}

func (state *loadState) readByte() (b byte, err error) {
	err = state.read(&b)
	return
}

func (state *loadState) readBool() (bool, error) {
	b, err := state.readByte()
	return b != 0, err
}

func (state *loadState) readCount() (int, error) {
	n, err := state.readInt()
	if err != nil {
		return 0, err
	} else if n < 0 {
		return 0, errCorrupted
	}
	return int(n), nil
}

func initialCapacity(n int) int {
	return min(n, undumpBatchSize)
}

func (state *loadState) readString() (s string, err error) {
	// Feel my pain
	maxUint := ^uint(0)
	var size uintptr
	var size64 uint64
	var size32 uint32
	if uint64(maxUint) == math.MaxUint64 {
		err = state.read(&size64)
		size = uintptr(size64)
	} else if maxUint == math.MaxUint32 {
		err = state.read(&size32)
		size = uintptr(size32)
	} else {
		panic(fmt.Sprintf("unsupported pointer size (%d)", maxUint))
	}
	if err != nil || size == 0 {
		return
	}
	capacity := uintptr(undumpBatchSize)
	if size < capacity {
		capacity = size
	}
	ba := make([]byte, 0, capacity)
	for uintptr(len(ba)) < size {
		start := len(ba)
		batch := size - uintptr(start)
		if batch > undumpBatchSize {
			batch = undumpBatchSize
		}
		ba = append(ba, make([]byte, batch)...)
		if err = state.read(ba[start:]); err != nil {
			return "", err
		}
	}
	s = string(ba[:len(ba)-1])
	return
}

func (state *loadState) readCode() (code []instruction, err error) {
	n, err := state.readCount()
	if err != nil || n == 0 {
		return
	}
	code = make([]instruction, 0, initialCapacity(n))
	for len(code) < n {
		start := len(code)
		code = append(code, make([]instruction, min(undumpBatchSize, n-start))...)
		if err = state.read(code[start:]); err != nil {
			return nil, err
		}
	}
	return
}

func (state *loadState) readUpValues() (u []upValueDesc, err error) {
	n, err := state.readCount()
	if err != nil || n == 0 {
		return
	}
	u = make([]upValueDesc, 0, initialCapacity(n))
	for i := 0; i < n; i++ {
		var v struct{ IsLocal, Index byte }
		if err = state.read(&v); err != nil {
			return nil, err
		}
		u = append(u, upValueDesc{isLocal: v.IsLocal != 0, index: int(v.Index)})
	}
	return
}

func (state *loadState) readLocalVariables() (localVariables []localVariable, err error) {
	var n int
	if n, err = state.readCount(); err != nil || n == 0 {
		return
	}
	localVariables = make([]localVariable, 0, initialCapacity(n))
	for i := 0; i < n; i++ {
		var v localVariable
		if v.name, err = state.readString(); err != nil {
			return nil, err
		}
		if v.startPC, err = state.readPC(); err != nil {
			return nil, err
		}
		if v.endPC, err = state.readPC(); err != nil {
			return nil, err
		}
		localVariables = append(localVariables, v)
	}
	return
}

func (state *loadState) readLineInfo() (lineInfo []int32, err error) {
	var n int
	if n, err = state.readCount(); err != nil || n == 0 {
		return
	}
	lineInfo = make([]int32, 0, initialCapacity(n))
	for len(lineInfo) < n {
		start := len(lineInfo)
		lineInfo = append(lineInfo, make([]int32, min(undumpBatchSize, n-start))...)
		if err = state.read(lineInfo[start:]); err != nil {
			return nil, err
		}
	}
	return
}

func (state *loadState) readDebug(p *prototype) (source string, lineInfo []int32, localVariables []localVariable, names []string, err error) {
	var n int
	if source, err = state.readString(); err != nil {
		return
	}
	if lineInfo, err = state.readLineInfo(); err != nil {
		return
	}
	if localVariables, err = state.readLocalVariables(); err != nil {
		return
	}
	if n, err = state.readCount(); err != nil {
		return
	} else if n > len(p.upValues) {
		err = errCorrupted
		return
	}
	names = make([]string, 0, initialCapacity(n))
	for i := 0; i < n; i++ {
		var name string
		if name, err = state.readString(); err != nil {
			return
		}
		names = append(names, name)
	}
	return
}

func (state *loadState) readConstants() (constants []value, prototypes []prototype, err error) {
	var n int
	if n, err = state.readCount(); err != nil || n == 0 {
		return
	}

	constants = make([]value, 0, initialCapacity(n))
	for i := 0; i < n; i++ {
		var c value
		var t byte
		switch t, err = state.readByte(); {
		case err != nil:
			return
		case t == byte(TypeNil):
			c = nil
		case t == byte(TypeBoolean):
			c, err = state.readBool()
		case t == byte(TypeNumber):
			c, err = state.readNumber()
		case t == byte(TypeString):
			c, err = state.readString()
		default:
			err = errUnknownConstantType
		}
		if err != nil {
			return
		}
		constants = append(constants, c)
	}
	return
}

func (state *loadState) readPrototypes() (prototypes []prototype, err error) {
	var n int
	if n, err = state.readCount(); err != nil || n == 0 {
		return
	}
	prototypes = make([]prototype, 0, initialCapacity(n))
	for i := 0; i < n; i++ {
		var p prototype
		if p, err = state.readFunction(); err != nil {
			return nil, err
		}
		prototypes = append(prototypes, p)
	}
	return
}

func (state *loadState) readFunction() (p prototype, err error) {
	if state.depth++; state.depth > maxUndumpNesting {
		return p, errCorrupted
	}
	defer func() { state.depth-- }()
	var n int32
	if n, err = state.readInt(); err != nil {
		return
	}
	p.lineDefined = int(n)
	if n, err = state.readInt(); err != nil {
		return
	}
	p.lastLineDefined = int(n)
	var b byte
	if b, err = state.readByte(); err != nil {
		return
	}
	p.parameterCount = int(b)
	if b, err = state.readByte(); err != nil {
		return
	}
	p.isVarArg = b != 0
	if b, err = state.readByte(); err != nil {
		return
	}
	p.maxStackSize = int(b)
	if p.maxStackSize < p.parameterCount {
		return p, errCorrupted
	}
	if p.code, err = state.readCode(); err != nil {
		return
	}
	if len(p.code) > 0 && p.maxStackSize == 0 {
		return p, errCorrupted
	}
	if p.constants, p.prototypes, err = state.readConstants(); err != nil {
		return
	}
	if p.prototypes, err = state.readPrototypes(); err != nil {
		return
	}
	if p.upValues, err = state.readUpValues(); err != nil {
		return
	}
	var names []string
	if p.source, p.lineInfo, p.localVariables, names, err = state.readDebug(&p); err != nil {
		return
	}
	for i, name := range names {
		p.upValues[i].name = name
	}
	return
}

func init() {
	copy(header.Signature[:], Signature)
	header.Version = VersionMajor<<4 | VersionMinor
	header.Format = 0
	if endianness() == binary.LittleEndian {
		header.Endianness = 1
	} else {
		header.Endianness = 0
	}
	header.IntSize = 4
	header.PointerSize = byte(1+^uintptr(0)>>32&1) * 4
	header.InstructionSize = byte(1+^instruction(0)>>32&1) * 4
	header.NumberSize = 8
	header.IntegralNumber = 0
	tail := "\x19\x93\r\n\x1a\n"
	copy(header.Tail[:], tail)

	// The uintptr numeric type is implementation-specific
	uintptrBitCount := byte(0)
	for bits := ^uintptr(0); bits != 0; bits >>= 1 {
		uintptrBitCount++
	}
	if uintptrBitCount != header.PointerSize*8 {
		panic(fmt.Sprintf("invalid pointer size (%d)", uintptrBitCount))
	}
}

func endianness() binary.ByteOrder {
	if x := 1; *(*byte)(unsafe.Pointer(&x)) == 1 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func (state *loadState) checkHeader() error {
	h := header
	if err := state.read(&h); err != nil {
		return err
	} else if h == header {
		return nil
	} else if string(h.Signature[:]) != Signature {
		return errNotPrecompiledChunk
	} else if h.Version != header.Version || h.Format != header.Format {
		return errVersionMismatch
	} else if h.Tail != header.Tail {
		return errCorrupted
	}
	return errIncompatible
}

func (l *State) undump(in io.Reader, name string) (c *luaClosure, err error) {
	if name[0] == '@' || name[0] == '=' {
		name = name[1:]
	} else if name[0] == Signature[0] {
		name = "binary string"
	}
	// TODO assign name to p.source?
	s := &loadState{in: in, order: endianness()}
	var p prototype
	if err = s.checkHeader(); err != nil {
		return
	} else if p, err = s.readFunction(); err != nil {
		return
	}
	c = l.newLuaClosure(&p)
	l.push(c)
	return
}
