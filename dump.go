package lua

import (
	"encoding/binary"
	"fmt"
	"io"
)

type dumpState struct {
	l     *State
	out   io.Writer
	order binary.ByteOrder
	err   error
}

func (d *dumpState) write(data interface{}) {
	if d.err == nil {
		d.err = binary.Write(d.out, d.order, data)
	}
}

func (d *dumpState) writeInt(i int) {
	d.write(int32(i))
}

func (d *dumpState) writePC(p pc) {
	d.writeInt(int(p))
}

func (d *dumpState) writeCode(p *prototype) {
	d.writeInt(len(p.code))
	d.write(p.code)
}

func (d *dumpState) writeByte(b byte) {
	d.write(b)
}

func (d *dumpState) writeBool(b bool) {
	if b {
		d.writeByte(1)
	} else {
		d.writeByte(0)
	}
}

func (d *dumpState) writeNumber(f float64) {
	d.write(f)
}

func (d *dumpState) writeConstants(p *prototype) {
	d.writeInt(len(p.constants))

	for _, o := range p.constants {
		d.writeByte(byte(d.l.valueToType(o)))

		switch o := o.(type) {
		case nil:
		case bool:
			d.writeBool(o)
		case int:
			d.writeInt(o)
		case string:
			d.writeString(o)
		default:
			d.l.assert(false)
		}
	}

	d.writeInt(len(p.prototypes))

	for _, o := range p.prototypes {
		d.dumpFunction(&o)
	}
}

func (d *dumpState) writeUpvalues(p *prototype) {
	d.writeInt(len(p.upValues))

	for _, u := range p.upValues {
		d.writeBool(u.isLocal)
		d.writeByte(byte(u.index))
	}
}

func (d *dumpState) writeString(s string) {
	switch header.PointerSize {
	case 8:
		d.write(uint64(len(s)))
	case 4:
		d.write(uint32(len(s)))
	default:
		panic(fmt.Sprintf("unsupported pointer size (%d)"))
	}
}

func (d *dumpState) writeDebug(p *prototype) {
	var n int
	d.writeString(p.source)
	d.writeInt(n)
	d.write(p.lineInfo)
	d.writeInt(n)

	for _, lv := range p.localVariables {
		d.writeString(lv.name)
		d.writePC(lv.startPC)
		d.writePC(lv.endPC)
	}

	d.writeInt(n)

	for _, uv := range p.upValues {
		d.writeString(uv.name)
	}
}

func (d *dumpState) dumpFunction(p *prototype) {
	d.writeInt(p.lineDefined)
	d.writeInt(p.lastLineDefined)
	d.writeByte(byte(p.parameterCount))
	d.writeBool(p.isVarArg)
	d.writeByte(byte(p.maxStackSize))
	d.writeCode(p)
	d.writeConstants(p)
	d.writeUpvalues(p)
	d.writeDebug(p)

}

func (d *dumpState) dumpHeader() {
	//enfianness is 0 or 1 - need to figure that out
	d.err = binary.Write(d.out, endianness(), header)
}

func (l *State) dump(p *prototype, w io.Writer) error {
	d := dumpState{l: l, out: w, order: endianness()}
	d.dumpHeader()
	d.dumpFunction(p)

	return d.err
}
