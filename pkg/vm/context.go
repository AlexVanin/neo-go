package vm

import (
	"encoding/binary"
)

// Context represent the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The raw program script.
	prog []byte

	// Breakpoints
	breakPoints []int
}

// NewContext return a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		ip:          -1,
		prog:        b,
		breakPoints: []int{},
	}
}

// Next return the next instruction to execute.
func (c *Context) Next() Instruction {
	c.ip++
	if c.ip >= len(c.prog) {
		return RET
	}
	return Instruction(c.prog[c.ip])
}

// IP returns the absolute instruction without taking 0 into account.
// If that program starts the ip = 0 but IP() will return 1, cause its
// the first instruction.
func (c *Context) IP() int {
	return c.ip + 1
}

// LenInstr returns the number of instructions loaded.
func (c *Context) LenInstr() int {
	return len(c.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, Instruction) {
	if c.ip < 0 {
		return c.ip, NOP
	}
	return c.ip, Instruction(c.prog[c.ip])
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	return &Context{
		ip:          c.ip,
		prog:        c.prog,
		breakPoints: c.breakPoints,
	}
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.prog
}

// Value implements StackItem interface.
func (c *Context) Value() interface{} {
	return c
}

func (c *Context) atBreakPoint() bool {
	for _, n := range c.breakPoints {
		if n == c.ip {
			return true
		}
	}
	return false
}

func (c *Context) String() string {
	return "execution context"
}

func (c *Context) readUint32() uint32 {
	start, end := c.IP(), c.IP()+4
	if end > len(c.prog) {
		panic("failed to read uint32 parameter")
	}
	val := binary.LittleEndian.Uint32(c.prog[start:end])
	c.ip += 4
	return val
}

func (c *Context) readUint16() uint16 {
	start, end := c.IP(), c.IP()+2
	if end > len(c.prog) {
		panic("failed to read uint16 parameter")
	}
	val := binary.LittleEndian.Uint16(c.prog[start:end])
	c.ip += 2
	return val
}

func (c *Context) readByte() byte {
	return c.readBytes(1)[0]
}

func (c *Context) readBytes(n int) []byte {
	start, end := c.IP(), c.IP()+n
	if end > len(c.prog) {
		return nil
	}

	out := make([]byte, n)
	copy(out, c.prog[start:end])
	c.ip += n
	return out
}

func (c *Context) readVarBytes() []byte {
	n := c.readByte()
	return c.readBytes(int(n))
}
