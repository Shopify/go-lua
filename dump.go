package lua

import (
  "encoding/binary"
  "io"
)

type dumpState struct {
  out io.Writer
  err error
}

func (d *dumpState) dumpFunction(p *prototype) {

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
