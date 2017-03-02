// test dump header
// create a dump state
// out is a local byte buffer (ignore other fields)
// call dump header
// call in undump check header
// reset byte buffer and create a load state taht wraps that with an order htat is endianness and call check header function on it
// validates what you wrote you can read in successfuly

package lua

import (
  "testing"
  "bytes"
  "encoding/binary"
)

func TestDump(t *testing.T) {
  out := new(bytes.Buffer)
  err := binary.Write(out, endianness(), "hi")

  d := dumpState{out, err}

  d.dumpHeader()

  l := loadState{out, endianness()}
  l.checkHeader()
}
