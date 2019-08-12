package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

type stackInfo func(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error)

var opFunc = map[stack.Instruction]stackInfo{
	stack.TUCK:            TUCK,
	stack.SWAP:            SWAP,
	stack.ROT:             ROT,
	stack.ROLL:            ROLL,
	stack.PICK:            PICK,
	stack.OVER:            OVER,
	stack.NIP:             NIP,
	stack.DUP:             DUP,
	stack.DROP:            DROP,
	stack.DEPTH:           DEPTH,
	stack.XTUCK:           XTUCK,
	stack.XSWAP:           XSWAP,
	stack.XDROP:           XDROP,
	stack.FROMALTSTACK:    FROMALTSTACK,
	stack.TOALTSTACK:      TOALTSTACK,
	stack.DUPFROMALTSTACK: DUPFROMALTSTACK,
	stack.JMPIFNOT:        JMPIFNOT,
	stack.JMPIF:           JMPIF,
	stack.JMP:             JMP,
	stack.NOP:             NOP,
	stack.HASH256:         HASH256,
	stack.HASH160:         HASH160,
	stack.SHA256:          SHA256,
	stack.SHA1:            SHA1,
	stack.XOR:             Xor,
	stack.OR:              Or,
	stack.AND:             And,
	stack.INVERT:          Invert,
	stack.MIN:             Min,
	stack.MAX:             Max,
	stack.WITHIN:          Within,
	stack.NUMEQUAL:        NumEqual,
	stack.NUMNOTEQUAL:     NumNotEqual,
	stack.BOOLAND:         BoolAnd,
	stack.BOOLOR:          BoolOr,
	stack.LT:              Lt,
	stack.LTE:             Lte,
	stack.GT:              Gt,
	stack.GTE:             Gte,
	stack.SHR:             Shr,
	stack.SHL:             Shl,
	stack.INC:             Inc,
	stack.DEC:             Dec,
	stack.DIV:             Div,
	stack.MOD:             Mod,
	stack.NZ:              Nz,
	stack.MUL:             Mul,
	stack.ABS:             Abs,
	stack.NOT:             Not,
	stack.SIGN:            Sign,
	stack.NEGATE:          Negate,
	stack.ADD:             Add,
	stack.SUB:             Sub,
	stack.PUSHBYTES1:      PushNBytes,
	stack.PUSHBYTES75:     PushNBytes,
	stack.RET:             RET,
	stack.EQUAL:           EQUAL,
	stack.THROWIFNOT:      THROWIFNOT,
	stack.THROW:           THROW,
}

func init() {
	for i := int(stack.PUSHBYTES1); i <= int(stack.PUSHBYTES75); i++ {
		opFunc[stack.Instruction(i)] = PushNBytes
	}
}
