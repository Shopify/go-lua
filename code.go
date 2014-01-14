package lua

const (
	oprMinus = iota
	oprNot
	oprLength
	oprNoUnary
)

const (
	noJump     = -1
	noRegister = maxArgA
)

const (
	oprAdd = iota
	oprSub
	oprMul
	oprDiv
	oprMod
	oprPow
	oprConcat
	oprEq
	oprLT
	oprLE
	oprNE
	oprGT
	oprGE
	oprAnd
	oprOr
	oprNoBinary
)

type function struct {
	f                      *prototype
	h                      *table
	previous               *function
	p                      *parser
	pc, jumpPC, lastTarget int
	isVarArg               bool
	bytecode               []instruction
	// TODO ...
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func (f *function) assert(cond bool) { f.p.l.assert(cond) }

func (f *function) code(i instruction) int {
	f.dischargeJumpPC()
	f.f.code = append(f.f.code, i) // TODO check that we always only append
	f.f.lineInfo = append(f.f.lineInfo, int32(f.p.lastLine))
	f.pc++
	return f.pc
}

func (f *function) codeABC(op opCode, a, b, c int) int {
	f.assert(opMode(op) == iABC)
	f.assert(bMode(op) != opArgN || b == 0)
	f.assert(cMode(op) != opArgN || c == 0)
	f.assert(a <= maxArgA && b <= maxArgB && c <= maxArgC)
	return f.code(createABC(op, a, b, c))
}

func (f *function) fixJump(pc, dest int) {
	f.assert(dest != noJump)
	offset := dest - (pc + 1)
	if abs(offset) > maxArgSBx {
		f.p.syntaxError("control structure too long")
	}
	f.bytecode[pc].setSBx(offset)
}

func (f *function) label() int {
	f.lastTarget = f.pc
	return f.pc
}

func (f *function) jump(pc int) int {
	if offset := f.bytecode[pc].sbx(); offset != noJump {
		return pc + 1 + offset
	}
	return noJump
}

func (f *function) jumpControl(pc int) *instruction {
	if pc >= 1 && testTMode(f.bytecode[pc-1].opCode()) {
		return &f.bytecode[pc-1]
	}
	return &f.bytecode[pc]
}

func (f *function) patchTestRegister(node, register int) bool {
	if i := f.jumpControl(node); i.opCode() != opTestSet {
		return false
	} else if register != noRegister && register != i.b() {
		i.setA(register)
	} else {
		*i = createABC(opTest, i.b(), 0, i.c())
	}
	return true
}

func (f *function) patchListHelper(list, target, register, defaultTarget int) {
	for list != noJump {
		next := f.jump(list)
		if f.patchTestRegister(list, register) {
			f.fixJump(list, target)
		} else {
			f.fixJump(list, defaultTarget)
		}
		list = next
	}
}

func (f *function) dischargeJumpPC() {
	f.patchListHelper(f.jumpPC, f.pc, noRegister, f.pc)
	f.jumpPC = noJump
}

func (f *function) patchList(list, target int) {
	if target == f.pc {
		f.patchToHere(list)
	} else {
		f.assert(target < f.pc)
		f.patchListHelper(list, target, noRegister, target)
	}
}

func (f *function) patchClose(list, level int) {
	for level, next := level+1, 0; list != noJump; list = next {
		next = f.jump(list)
		f.assert(f.bytecode[list].opCode() == opJump && f.bytecode[list].a() == 0 || f.bytecode[list].a() >= level)
		f.bytecode[list].setA(level)
	}
}

func (f *function) patchToHere(list int) {
	f.label()
	f.concat(&f.jumpPC, list)
}

func (f *function) concat(l1 *int, l2 int) {
	switch {
	case l2 == noJump:
	case *l1 == noJump:
		*l1 = l2
	default:
		list := *l1
		for next := f.jump(list); next != noJump; list, next = next, f.jump(next) {
		}
		f.fixJump(list, l2)
	}
}

// func (f *function) expressionToRegisterOrConstant(e exprDesc) exprDesc {
//   v := f.expressionToValue(e)
//   switch v.kind {
//   case kindTrue:

//     , kindFalse, kindNil:
//     if
//   }
// }

// int luaK_exp2RK (FuncState *fs, expdesc *e) {
//   luaK_exp2val(fs, e);
//   switch (e->k) {
//     case VTRUE:
//     case VFALSE:
//     case VNIL: {
//       if (fs->nk <= MAXINDEXRK) {  /* constant fits in RK operand? */
//         e->u.info = (e->k == VNIL) ? nilK(fs) : boolK(fs, (e->k == VTRUE));
//         e->k = VK;
//         return RKASK(e->u.info);
//       }
//       else break;
//     }
//     case VKNUM: {
//       e->u.info = luaK_numberK(fs, e->u.nval);
//       e->k = VK;
//       /* go through */
//     }
//     case VK: {
//       if (e->u.info <= MAXINDEXRK)  /* constant fits in argC? */
//         return RKASK(e->u.info);
//       else break;
//     }
//     default: break;
//   }
//   /* not a constant in the right range: put it in a register */
//   return luaK_exp2anyreg(fs, e);
// }

// func (f *function) expressionToRegisterOrConstant(e exprDesc) int {

// }

func (f *function) index(t, k exprDesc) (r exprDesc) {
	// TODO assert(!t.hasJumps())
	// r.table = t.info
	// r.index = f.expressionToRegisterOrConstant(k)
	// if t.kind == kindUpValue {
	// 	r.tableType = kindUpValue
	// } else {
	// 	r.tableType = checkExpression(xxx, kindLocal)
	// }
	// r.kind = kindIndexed
	return
}
