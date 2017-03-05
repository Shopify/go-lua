package lua

import (
  "encoding/binary"
  "io"
)

type dumpState struct {
  out io.Writer
  err error
}

func (d *dumpState) writeInt(i int32) {
  d.write(i)
}

func (d *dumpState) writeChar(i int32) {
  x := rune(i)
  d.write(x)
}

func (d *dumpState) writeCode(p *prototype) {
  //d.writeInt(p.sizecode)
  d.write(p.code)
}

func (d *dumpState) writeByte(b byte) {
  d.write(b)
}

func (d *dumpState) writeBool(b bool) {
  d.writeByte(b)
}

func (d *dumpState) writeNumber(f float64) {
  d.write(f)
}

func (d *dumpState) writeString(s string) {

}

func (d *dumpState) writeConstants(p *prototype) {
  for i := range p.constants {
    var t byte
    switch t, err = d.writeByte(t); {
    case t == byte(TypeNil):
      constants[i] = nil
    case t == byte(TypeBoolean):
      constants[i], err = state.readBool()
    case t == byte(TypeNumber):
      constants[i], err = state.readNumber()
    case t == byte(TypeString):
      constants[i], err = state.readString()
    default:
      err = errUnknownConstantType
    }
    if err != nil {
      return
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
  d.writeChar(p.isVarArg)
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
