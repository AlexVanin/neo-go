package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testNative struct {
	meta   interop.ContractMD
	blocks chan uint32
}

func (tn *testNative) Initialize(_ *interop.Context) error {
	return nil
}

func (tn *testNative) Metadata() *interop.ContractMD {
	return &tn.meta
}

func (tn *testNative) OnPersist(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	if ic.Trigger != trigger.OnPersist {
		panic("invalid trigger")
	}
	select {
	case tn.blocks <- ic.Block.Index:
		return stackitem.NewBool(true)
	default:
		return stackitem.NewBool(false)
	}
}

var _ interop.Contract = (*testNative)(nil)

// registerNative registers native contract in the blockchain.
func (bc *Blockchain) registerNative(c interop.Contract) {
	bc.contracts.Contracts = append(bc.contracts.Contracts, c)
}

const testSumPrice = 1000000

func newTestNative() *testNative {
	tn := &testNative{
		meta:   *interop.NewContractMD("Test.Native.Sum"),
		blocks: make(chan uint32, 1),
	}
	desc := &manifest.Method{
		Name: "sum",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("addend1", smartcontract.IntegerType),
			manifest.NewParameter("addend2", smartcontract.IntegerType),
		},
		ReturnType: smartcontract.IntegerType,
	}
	md := &interop.MethodAndPrice{
		Func:          tn.sum,
		Price:         testSumPrice,
		RequiredFlags: smartcontract.NoneFlag,
	}
	tn.meta.AddMethod(md, desc, true)

	desc = &manifest.Method{
		Name: "callOtherContractNoReturn",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("contractHash", smartcontract.Hash160Type),
			manifest.NewParameter("method", smartcontract.StringType),
			manifest.NewParameter("arg", smartcontract.ArrayType),
		},
		ReturnType: smartcontract.VoidType,
	}
	md = &interop.MethodAndPrice{
		Func:          tn.callOtherContractNoReturn,
		Price:         testSumPrice,
		RequiredFlags: smartcontract.NoneFlag}
	tn.meta.AddMethod(md, desc, true)

	desc = &manifest.Method{Name: "onPersist", ReturnType: smartcontract.BoolType}
	md = &interop.MethodAndPrice{Func: tn.OnPersist, RequiredFlags: smartcontract.AllowModifyStates}
	tn.meta.AddMethod(md, desc, false)

	return tn
}

func (tn *testNative) sum(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	s1, err := args[0].TryInteger()
	if err != nil {
		panic(err)
	}
	s2, err := args[1].TryInteger()
	if err != nil {
		panic(err)
	}
	return stackitem.NewBigInteger(s1.Add(s1, s2))
}

func toUint160(item stackitem.Item) util.Uint160 {
	bs, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	u, err := util.Uint160DecodeBytesBE(bs)
	if err != nil {
		panic(err)
	}
	return u
}

func (tn *testNative) call(ic *interop.Context, args []stackitem.Item, retState vm.CheckReturnState) {
	cs, err := ic.DAO.GetContractState(toUint160(args[0]))
	if err != nil {
		panic(err)
	}
	bs, err := args[1].TryBytes()
	if err != nil {
		panic(err)
	}
	err = contract.CallExInternal(ic, cs, string(bs), args[2].Value().([]stackitem.Item), smartcontract.All, retState, nil)
	if err != nil {
		panic(err)
	}
}

func (tn *testNative) callOtherContractNoReturn(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	tn.call(ic, args, vm.EnsureIsEmpty)
	return stackitem.Null{}
}

func TestNativeContract_Invoke(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	// System.Contract.Call + "sum" itself + opcodes for pushing arguments (PACK is 15000)
	res, err := invokeContractMethod(chain, testSumPrice*2+18000, tn.Metadata().Hash, "sum", int64(14), int64(28))
	require.NoError(t, err)
	checkResult(t, res, stackitem.Make(42))
	require.NoError(t, chain.persist())

	select {
	case index := <-tn.blocks:
		require.Equal(t, chain.blockHeight, index)
	default:
		require.Fail(t, "onPersist wasn't called")
	}

	// Enough for Call and other opcodes, but not enough for "sum" call.
	res, err = invokeContractMethod(chain, testSumPrice*2+8000, tn.Metadata().Hash, "sum", int64(14), int64(28))
	require.NoError(t, err)
	checkFAULTState(t, res)

}

func TestNativeContract_InvokeInternal(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.config.StateRootInHeader)
	ic := chain.newInteropContext(trigger.Application, d, nil, nil)
	v := ic.SpawnVM()

	t.Run("fail, bad current script hash", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, util.Uint160{1, 2, 3}, smartcontract.All)
		v.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.NewBigInteger(big.NewInt(14)), stackitem.NewBigInteger(big.NewInt(28))}))
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().Name)

		// it's prohibited to call natives directly
		require.Error(t, native.Call(ic))
	})

	t.Run("success", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, tn.Metadata().Hash, smartcontract.All)
		v.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.NewBigInteger(big.NewInt(14)), stackitem.NewBigInteger(big.NewInt(28))}))
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().Name)

		require.NoError(t, native.Call(ic))

		value := v.Estack().Pop().BigInt()
		require.Equal(t, int64(42), value.Int64())
	})
}

func TestNativeContract_InvokeOtherContract(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	cs, _ := getTestContractState()
	require.NoError(t, chain.dao.PutContractState(cs))

	t.Run("non-native, no return", func(t *testing.T) {
		res, err := invokeContractMethod(chain, testSumPrice*4+10000, tn.Metadata().Hash, "callOtherContractNoReturn", cs.ScriptHash(), "justReturn", []interface{}{})
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{}) // simple call is done with EnsureNotEmpty
	})
}
