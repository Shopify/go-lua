// validates what you wrote you can read in successfuly

package lua

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDump(t *testing.T) {
	out := new(bytes.Buffer)
	err := binary.Write(out, endianness(), "hi")

	d := dumpState{out, endianness(), err}

	d.dumpHeader()

	l := loadState{out, endianness()}
	l.checkHeader()
}

//undump and then dump and check if the same
//os.open and then readall so it is in a buffer to make comparison easier
