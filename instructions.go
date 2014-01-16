package lua

type opCode uint

const (
	iABC int = iota
	iABx
	iAsBx
	iAx
)

const (
	opMove opCode = iota
	opLoadConstant
	opLoadConstantEx
	opLoadBool
	opLoadNil
	opGetUpValue
	opGetTableUp
	opGetTable
	opSetTableUp
	opSetUpValue
	opSetTable
	opNewTable
	opSelf
	opAdd
	opSub
	opMul
	opDiv
	opMod
	opPow
	opUnaryMinus
	opNot
	opLength
	opConcat
	opJump
	opEqual
	opLessThan
	opLessOrEqual
	opTest
	opTestSet
	opCall
	opTailCall
	opReturn
	opForLoop
	opForPrep
	opTForCall
	opTForLoop
	opSetList
	opClosure
	opVarArg
	opExtraArg
)

var opNames = []string{
	"MOVE",
	"LOADK",
	"LOADKX",
	"LOADBOOL",
	"LOADNIL",
	"GETUPVAL",
	"GETTABUP",
	"GETTABLE",
	"SETTABUP",
	"SETUPVAL",
	"SETTABLE",
	"NEWTABLE",
	"SELF",
	"ADD",
	"SUB",
	"MUL",
	"DIV",
	"MOD",
	"POW",
	"UNM",
	"NOT",
	"LEN",
	"CONCAT",
	"JMP",
	"EQ",
	"LT",
	"LE",
	"TEST",
	"TESTSET",
	"CALL",
	"TAILCALL",
	"RETURN",
	"FORLOOP",
	"FORPREP",
	"TFORCALL",
	"TFORLOOP",
	"SETLIST",
	"CLOSURE",
	"VARARG",
	"EXTRAARG",
}

const (
	sizeC             = 9
	sizeB             = 9
	sizeBx            = sizeC + sizeB
	sizeA             = 8
	sizeAx            = sizeC + sizeB + sizeA
	sizeOp            = 6
	posOp             = 0
	posA              = posOp + sizeOp
	posC              = posA + sizeA
	posB              = posC + sizeC
	posBx             = posC
	posAx             = posA
	bitRK             = 1 << (sizeB - 1)
	maxIndexRK        = bitRK - 1
	maxArgAx          = 1<<sizeAx - 1
	maxArgBx          = 1<<sizeBx - 1
	maxArgSBx         = maxArgBx >> 1 // sBx is signed
	maxArgA           = 1<<sizeA - 1
	maxArgB           = 1<<sizeB - 1
	maxArgC           = 1<<sizeC - 1
	listItemsPerFlush = 50 // # list items to accumulate before a setList instruction
)

type instruction uint32

func isConstant(x int) bool   { return 0 != x&bitRK }
func constantIndex(r int) int { return r & ^bitRK }
func asConstant(r int) int    { return r | bitRK }

// creates a mask with 'n' 1 bits at position 'p'
func mask1(n, p uint) instruction { return ^(^instruction(0) << n) << p }

// creates a mask with 'n' 0 bits at position 'p'
func mask0(n, p uint) instruction { return ^mask1(n, p) }

func (i instruction) opCode() opCode         { return opCode(i >> posOp & mask1(sizeOp, 0)) }
func (i instruction) arg(pos, size uint) int { return int(i >> pos & mask1(size, 0)) }
func (i *instruction) setOpCode(op opCode)   { i.setArg(posOp, sizeOp, int(op)) }
func (i *instruction) setArg(pos, size uint, arg int) {
	*i = *i&mask0(size, pos) | instruction(arg)<<pos&mask1(size, pos)
}

func (i instruction) a() int   { return i.arg(posA, sizeA) }
func (i instruction) b() int   { return i.arg(posB, sizeB) }
func (i instruction) c() int   { return i.arg(posC, sizeC) }
func (i instruction) bx() int  { return i.arg(posBx, sizeBx) }
func (i instruction) ax() int  { return i.arg(posAx, sizeAx) }
func (i instruction) sbx() int { return i.bx() - maxArgSBx }

func (i instruction) setA(arg int)   { i.setArg(posA, sizeA, arg) }
func (i instruction) setB(arg int)   { i.setArg(posB, sizeB, arg) }
func (i instruction) setC(arg int)   { i.setArg(posC, sizeC, arg) }
func (i instruction) setBx(arg int)  { i.setArg(posBx, sizeBx, arg) }
func (i instruction) setAx(arg int)  { i.setArg(posAx, sizeAx, arg) }
func (i instruction) setSBx(arg int) { i.setArg(posBx, sizeBx, arg+maxArgSBx) }

func createABC(op opCode, a, b, c int) instruction {
	return instruction(op)<<posOp |
		instruction(a)<<posA |
		instruction(b)<<posB |
		instruction(c)<<posC
}

func createABx(op opCode, a, bx int) instruction {
	return instruction(op)<<posOp |
		instruction(a)<<posA |
		instruction(bx)<<posBx
}

func createAx(op opCode, a int) instruction { return instruction(op)<<posOp | instruction(a)<<posAx }

func opmode(t, a, b, c, m int) byte { return byte(t<<7 | a<<6 | b<<4 | c<<2 | m) }

const (
	opArgN = iota // argument is not used
	opArgU        // argument is used
	opArgR        // argument is a register or a jump offset
	opArgK        // argument is a constant or register/constant
)

func opMode(m opCode) int     { return int(opModes[m] & 3) }
func bMode(m opCode) byte     { return (opModes[m] >> 4) & 3 }
func cMode(m opCode) byte     { return (opModes[m] >> 2) & 3 }
func testAMode(m opCode) bool { return opModes[m]&(1<<6) != 0 }
func testTMode(m opCode) bool { return opModes[m]&(1<<7) != 0 }

var opModes []byte = []byte{
	//     T  A    B       C     mode		    opcode
	opmode(0, 1, opArgR, opArgN, iABC),  // opMove
	opmode(0, 1, opArgK, opArgN, iABx),  // opLoadConstant
	opmode(0, 1, opArgN, opArgN, iABx),  // opLoadConstantEx
	opmode(0, 1, opArgU, opArgU, iABC),  // opLoadBool
	opmode(0, 1, opArgU, opArgN, iABC),  // opLoadNil
	opmode(0, 1, opArgU, opArgN, iABC),  // opGetUpValue
	opmode(0, 1, opArgU, opArgK, iABC),  // opGetTableUp
	opmode(0, 1, opArgR, opArgK, iABC),  // opGetTable
	opmode(0, 0, opArgK, opArgK, iABC),  // opSetTableUp
	opmode(0, 0, opArgU, opArgN, iABC),  // opSetUpValue
	opmode(0, 0, opArgK, opArgK, iABC),  // opSetTable
	opmode(0, 1, opArgU, opArgU, iABC),  // opNewTable
	opmode(0, 1, opArgR, opArgK, iABC),  // opSelf
	opmode(0, 1, opArgK, opArgK, iABC),  // opAdd
	opmode(0, 1, opArgK, opArgK, iABC),  // opSub
	opmode(0, 1, opArgK, opArgK, iABC),  // opMul
	opmode(0, 1, opArgK, opArgK, iABC),  // opDiv
	opmode(0, 1, opArgK, opArgK, iABC),  // opMod
	opmode(0, 1, opArgK, opArgK, iABC),  // opPow
	opmode(0, 1, opArgR, opArgN, iABC),  // opUnaryMinus
	opmode(0, 1, opArgR, opArgN, iABC),  // opNot
	opmode(0, 1, opArgR, opArgN, iABC),  // opLength
	opmode(0, 1, opArgR, opArgR, iABC),  // opConcat
	opmode(0, 0, opArgR, opArgN, iAsBx), // opJump
	opmode(1, 0, opArgK, opArgK, iABC),  // opEqual
	opmode(1, 0, opArgK, opArgK, iABC),  // opLessThan
	opmode(1, 0, opArgK, opArgK, iABC),  // opLessOrEqual
	opmode(1, 0, opArgN, opArgU, iABC),  // opTest
	opmode(1, 1, opArgR, opArgU, iABC),  // opTestSet
	opmode(0, 1, opArgU, opArgU, iABC),  // opCall
	opmode(0, 1, opArgU, opArgU, iABC),  // opTailCall
	opmode(0, 0, opArgU, opArgN, iABC),  // opReturn
	opmode(0, 1, opArgR, opArgN, iAsBx), // opForLoop
	opmode(0, 1, opArgR, opArgN, iAsBx), // opForPrep
	opmode(0, 0, opArgN, opArgU, iABC),  // opTForCall
	opmode(0, 1, opArgR, opArgN, iAsBx), // opTForLoop
	opmode(0, 0, opArgU, opArgU, iABC),  // opSetList
	opmode(0, 1, opArgU, opArgN, iABx),  // opClosure
	opmode(0, 1, opArgU, opArgN, iABC),  // opVarArg
	opmode(0, 0, opArgU, opArgU, iAx),   // opExtraArg
}
