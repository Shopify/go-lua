package lua

import (
	"encoding/binary"
	"io"
)

type dumpState struct {
	out   io.Writer
	order binary.ByteOrder
	err   error
}

func (d *dumpState) write(data interface{}) error {
	return binary.Write(d.out, d.order, data)
}

func (d *dumpState) writeInt(i int) error {
	return d.write(i)
}

func (d *dumpState) writeChar(i int) error {
	x := rune(i)
	return d.write(x)
}

func (d *dumpState) writeCode(p *prototype) error {
	//d.writeInt(p.sizecode)
	return d.write(p.code)
}

func (d *dumpState) writeByte(b byte) error {
	return d.write(b)
}

func (d *dumpState) writeBool(b bool) error {
	return d.writeByte(b)
}

func (d *dumpState) writeNumber(f float64) error {
	return d.write(f)
}

func (d *dumpState) writeString(s string) error {
	return
}

func (d *dumpState) writeConstants(p *prototype) {
	for i := range p.constants {
		var n = len(p.constants)

		for i := 0; i < n; i++ {
			var o = p.constants[i]
			err := d.writeChar(i)

			switch i := o.(type) {
			case nil:
				break
			case bool:
				if i {
					err = d.writeChar(1)
				} else {
					err = d.writeChar(0)
				}
				break
			case int:
				err = d.writeInt(i)
				break
			case string:
				err = d.writeString(i)
				break
			default:
				err = errUnknownConstantType
			}
			if err != nil {
				return
			}
		}

		n = len(p)
		writeInt(n)

		for i = 0; i < n; i++ {
			dumpFunction(p)
		}
	}
}

func (d *dumpState) writeUpvalues(p *prototype) {

}

func (d *dumpState) writeDebug(p *prototype) {

}

func (d *dumpState) dumpFunction(p *prototype) (err error) {
	d.writeInt(p.lineDefined)
	d.writeInt(p.lastLineDefined)
	d.writeChar(p.parameterCount)

	if p.isVarArg {
		d.writeChar(1)
	} else {
		d.writeChar(0)
	}

	d.writeChar(p.maxStackSize)
	d.writeCode(p)
	d.writeConstants(p)
	d.writeUpvalues(p)
	d.writeDebug(p)

}

func (d *dumpState) dumpHeader() {
	d.err = binary.Write(d.out, endianness(), header)
}

func (l *State) dump(p *prototype, w io.Writer) error {
	d := dumpState{out: w}
	d.dumpHeader()
	d.dumpFunction(p)

	return d.err
}
