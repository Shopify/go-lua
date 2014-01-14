package lua

const (
	oprMinus = iota
	oprNot
	oprLength
	oprNoUnary
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
	f        *prototype
	h        *table
	previous *function
	p        *parser
	pc       int
	isVarArg bool
	// TODO ...
}

func (f *function) assert(cond bool) {
	f.p.l.assert(cond)
}

func (f *function) code(i instruction) int {
	// f.dischargeJumpPC()
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
