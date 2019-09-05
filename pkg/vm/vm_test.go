package vm

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestInteropHook(t *testing.T) {
	v := New(ModeMute)
	v.RegisterInteropFunc("foo", func(evm *VM) error {
		evm.Estack().PushVal(1)
		return nil
	})

	buf := new(bytes.Buffer)
	EmitSyscall(buf, "foo")
	EmitOpcode(buf, RET)
	v.Load(buf.Bytes())
	v.Run()
	assert.Equal(t, 1, v.estack.Len())
	assert.Equal(t, big.NewInt(1), v.estack.Pop().value.Value())
}

func TestRegisterInterop(t *testing.T) {
	v := New(ModeMute)
	currRegistered := len(v.interop)
	v.RegisterInteropFunc("foo", func(evm *VM) error { return nil })
	assert.Equal(t, currRegistered+1, len(v.interop))
	_, ok := v.interop["foo"]
	assert.Equal(t, true, ok)
}

func TestPushBytes1to75(t *testing.T) {
	buf := new(bytes.Buffer)
	for i := 1; i <= 75; i++ {
		b := randomBytes(i)
		EmitBytes(buf, b)
		vm := load(buf.Bytes())
		vm.Step()

		assert.Equal(t, 1, vm.estack.Len())

		elem := vm.estack.Pop()
		assert.IsType(t, &ByteArrayItem{}, elem.value)
		assert.IsType(t, elem.Bytes(), b)
		assert.Equal(t, 0, vm.estack.Len())

		vm.execute(nil, RET)

		assert.Equal(t, 0, vm.astack.Len())
		assert.Equal(t, 0, vm.istack.Len())
		buf.Reset()
	}
}

func TestPushm1to16(t *testing.T) {
	var prog []byte
	for i := int(PUSHM1); i <= int(PUSH16); i++ {
		if i == 80 {
			continue // opcode layout we got here.
		}
		prog = append(prog, byte(i))
	}

	vm := load(prog)
	for i := int(PUSHM1); i <= int(PUSH16); i++ {
		if i == 80 {
			continue // nice opcode layout we got here.
		}
		vm.Step()

		elem := vm.estack.Pop()
		assert.IsType(t, &BigIntegerItem{}, elem.value)
		val := i - int(PUSH1) + 1
		assert.Equal(t, elem.BigInt().Int64(), int64(val))
	}
}

func TestPushData1(t *testing.T) {

}

func TestPushData2(t *testing.T) {

}

func TestPushData4(t *testing.T) {

}

func TestAdd(t *testing.T) {
	prog := makeProgram(ADD)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, int64(6), vm.estack.Pop().BigInt().Int64())
}

func TestMul(t *testing.T) {
	prog := makeProgram(MUL)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, int64(8), vm.estack.Pop().BigInt().Int64())
}

func TestDiv(t *testing.T) {
	prog := makeProgram(DIV)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
}

func TestSub(t *testing.T) {
	prog := makeProgram(SUB)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
}

func TestLT(t *testing.T) {
	prog := makeProgram(LT)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestLTE(t *testing.T) {
	prog := makeProgram(LTE)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestGT(t *testing.T) {
	prog := makeProgram(GT)
	vm := load(prog)
	vm.estack.PushVal(9)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, true, vm.estack.Pop().Bool())

}

func TestGTE(t *testing.T) {
	prog := makeProgram(GTE)
	vm := load(prog)
	vm.estack.PushVal(3)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestDepth(t *testing.T) {
	prog := makeProgram(DEPTH)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, int64(3), vm.estack.Pop().BigInt().Int64())
}

func TestNumEqual(t *testing.T) {
	prog := makeProgram(NUMEQUAL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestNumNotEqual(t *testing.T) {
	prog := makeProgram(NUMNOTEQUAL)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestINC(t *testing.T) {
	prog := makeProgram(INC)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, big.NewInt(2), vm.estack.Pop().BigInt())
}

func TestAppCall(t *testing.T) {
	prog := []byte{byte(APPCALL)}
	hash := util.Uint160{}
	prog = append(prog, hash.Bytes()...)
	prog = append(prog, byte(RET))

	vm := load(prog)
	vm.scripts[hash] = makeProgram(DEPTH)
	vm.estack.PushVal(2)

	vm.Run()
	elem := vm.estack.Pop() // depth should be 1
	assert.Equal(t, int64(1), elem.BigInt().Int64())
}

func TestSimpleCall(t *testing.T) {
	progStr := "52c56b525a7c616516006c766b00527ac46203006c766b00c3616c756653c56b6c766b00527ac46c766b51527ac46203006c766b00c36c766b51c393616c7566"
	result := 12

	prog, err := hex.DecodeString(progStr)
	if err != nil {
		t.Fatal(err)
	}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, result, int(vm.estack.Pop().BigInt().Int64()))
}

func TestNZtrue(t *testing.T) {
	prog := makeProgram(NZ)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestNZfalse(t *testing.T) {
	prog := makeProgram(NZ)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func makeProgram(opcodes ...Instruction) []byte {
	prog := make([]byte, len(opcodes)+1) // RET
	for i := 0; i < len(opcodes); i++ {
		prog[i] = byte(opcodes[i])
	}
	prog[len(prog)-1] = byte(RET)
	return prog
}

func load(prog []byte) *VM {
	vm := New(ModeMute)
	vm.mute = true
	vm.istack.PushVal(NewContext(prog))
	return vm
}

func randomBytes(n int) []byte {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}
