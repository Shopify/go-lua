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
	maxArgBx          = 1<<sizeBx - 1
	maxArgSBx         = maxArgBx >> 1 // sBx is signed
	listItemsPerFlush = 50            // # list items to accumulate before a setList instruction
)

type instruction uint32

func isConstant(x int) bool {
	return 0 != x&bitRK
}

func constantIndex(r int) int {
	return r & ^bitRK
}

// creates a mask with 'n' 1 bits at position 'p'
func mask1(n, p uint) instruction {
	return ^(^instruction(0) << n) << p
}

// creates a mask with 'n' 0 bits at position 'p'
func mask0(n, p uint) instruction {
	return ^mask1(n, p)
}

func (i instruction) opCode() opCode {
	return opCode(i >> posOp & mask1(sizeOp, 0))
}

func (i instruction) arg(pos, size uint) int {
	return int(i >> pos & mask1(size, 0))
}

func (i instruction) a() int {
	return i.arg(posA, sizeA)
}

func (i instruction) b() int {
	return i.arg(posB, sizeB)
}

func (i instruction) c() int {
	return i.arg(posC, sizeC)
}

func (i instruction) bx() int {
	return i.arg(posBx, sizeBx)
}

func (i instruction) ax() int {
	return i.arg(posAx, sizeAx)
}

func (i instruction) sbx() int {
	return i.bx() - maxArgSBx
}

func createABC(op, a, b, c int) instruction {
	return instruction(op)<<posOp |
		instruction(a)<<posA |
		instruction(b)<<posB |
		instruction(c)<<posC
}

func createAbx(op, a, bx int) instruction {
	return instruction(op)<<posOp |
		instruction(a)<<posA |
		instruction(bx)<<posBx
}

func createAx(op, a int) instruction {
	return instruction(op)<<posOp | instruction(a)<<posAx
}
