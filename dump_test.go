
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
