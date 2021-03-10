package crypto

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initCHECKMULTISIG(isR1 bool, msg []byte, n int) ([]stackitem.Item, []stackitem.Item, map[string]*keys.PublicKey, error) {
	var err error

	keyMap := make(map[string]*keys.PublicKey)
	pkeys := make([]*keys.PrivateKey, n)
	pubs := make([]stackitem.Item, n)
	for i := range pubs {
		if isR1 {
			pkeys[i], err = keys.NewPrivateKey()
		} else {
			pkeys[i], err = keys.NewSecp256k1PrivateKey()
		}
		if err != nil {
			return nil, nil, nil, err
		}

		pk := pkeys[i].PublicKey()
		data := pk.Bytes()
		pubs[i] = stackitem.NewByteArray(data)
		keyMap[string(data)] = pk
	}

	sigs := make([]stackitem.Item, n)
	for i := range sigs {
		sig := pkeys[i].Sign(msg)
		sigs[i] = stackitem.NewByteArray(sig)
	}

	return pubs, sigs, keyMap, nil
}

func subSlice(arr []stackitem.Item, indices []int) []stackitem.Item {
	if indices == nil {
		return arr
	}

	result := make([]stackitem.Item, len(indices))
	for i, j := range indices {
		result[i] = arr[j]
	}

	return result
}

func initCheckMultisigVMNoArgs(isR1 bool) *vm.VM {
	buf := make([]byte, 5)
	buf[0] = byte(opcode.SYSCALL)
	if isR1 {
		binary.LittleEndian.PutUint32(buf[1:], ecdsaSecp256r1CheckMultisigID)
	} else {
		binary.LittleEndian.PutUint32(buf[1:], ecdsaSecp256k1CheckMultisigID)
	}

	ic := &interop.Context{Trigger: trigger.Verification}
	Register(ic)
	v := ic.SpawnVM()
	v.LoadScript(buf)
	return v
}

func initCHECKMULTISIGVM(t *testing.T, isR1 bool, n int, ik, is []int) *vm.VM {
	v := initCheckMultisigVMNoArgs(isR1)
	msg := []byte("NEO - An Open Network For Smart Economy")

	pubs, sigs, _, err := initCHECKMULTISIG(isR1, msg, n)
	require.NoError(t, err)

	pubs = subSlice(pubs, ik)
	sigs = subSlice(sigs, is)

	v.Estack().PushVal(sigs)
	v.Estack().PushVal(pubs)
	v.Estack().PushVal(msg)

	return v
}

func testCHECKMULTISIGGood(t *testing.T, isR1 bool, n int, is []int) {
	v := initCHECKMULTISIGVM(t, isR1, n, nil, is)

	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.True(t, v.Estack().Pop().Bool())
}

func TestECDSASecp256r1CheckMultisigGood(t *testing.T) {
	testCurveCHECKMULTISIGGood(t, true)
}

func TestECDSASecp256k1CheckMultisigGood(t *testing.T) {
	testCurveCHECKMULTISIGGood(t, false)
}

func testCurveCHECKMULTISIGGood(t *testing.T, isR1 bool) {
	t.Run("3_1", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 3, []int{1}) })
	t.Run("2_2", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 2, []int{0, 1}) })
	t.Run("3_3", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 3, []int{0, 1, 2}) })
	t.Run("3_2", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 3, []int{0, 2}) })
	t.Run("4_2", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 4, []int{0, 2}) })
	t.Run("10_7", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 10, []int{2, 3, 4, 5, 6, 8, 9}) })
	t.Run("12_9", func(t *testing.T) { testCHECKMULTISIGGood(t, isR1, 12, []int{0, 1, 4, 5, 6, 7, 8, 9}) })
}

func testCHECKMULTISIGBad(t *testing.T, isR1 bool, isErr bool, n int, ik, is []int) {
	v := initCHECKMULTISIGVM(t, isR1, n, ik, is)

	if isErr {
		require.Error(t, v.Run())
		return
	}
	require.NoError(t, v.Run())
	assert.Equal(t, 1, v.Estack().Len())
	assert.False(t, v.Estack().Pop().Bool())
}

func TestECDSASecp256r1CheckMultisigBad(t *testing.T) {
	testCurveCHECKMULTISIGBad(t, true)
}

func TestECDSASecp256k1CheckMultisigBad(t *testing.T) {
	testCurveCHECKMULTISIGBad(t, false)
}

func testCurveCHECKMULTISIGBad(t *testing.T, isR1 bool) {
	t.Run("1_1 wrong signature", func(t *testing.T) { testCHECKMULTISIGBad(t, isR1, false, 2, []int{0}, []int{1}) })
	t.Run("3_2 wrong order", func(t *testing.T) { testCHECKMULTISIGBad(t, isR1, false, 3, []int{0, 2}, []int{2, 0}) })
	t.Run("3_2 duplicate sig", func(t *testing.T) { testCHECKMULTISIGBad(t, isR1, false, 3, nil, []int{0, 0}) })
	t.Run("1_2 too many signatures", func(t *testing.T) { testCHECKMULTISIGBad(t, isR1, true, 2, []int{0}, []int{0, 1}) })
	t.Run("gas limit exceeded", func(t *testing.T) {
		v := initCHECKMULTISIGVM(t, isR1, 1, []int{0}, []int{0})
		v.GasLimit = fee.ECDSAVerifyPrice - 1
		require.Error(t, v.Run())
	})

	msg := []byte("NEO - An Open Network For Smart Economy")
	pubs, sigs, _, err := initCHECKMULTISIG(isR1, msg, 1)
	require.NoError(t, err)
	arr := stackitem.NewArray([]stackitem.Item{stackitem.NewArray(nil)})

	t.Run("invalid message type", func(t *testing.T) {
		v := initCheckMultisigVMNoArgs(isR1)
		v.Estack().PushVal(sigs)
		v.Estack().PushVal(pubs)
		v.Estack().PushVal(stackitem.NewArray(nil))
		require.Error(t, v.Run())
	})
	t.Run("invalid public keys", func(t *testing.T) {
		v := initCheckMultisigVMNoArgs(isR1)
		v.Estack().PushVal(sigs)
		v.Estack().PushVal(arr)
		v.Estack().PushVal(msg)
		require.Error(t, v.Run())
	})
	t.Run("invalid signatures", func(t *testing.T) {
		v := initCheckMultisigVMNoArgs(isR1)
		v.Estack().PushVal(arr)
		v.Estack().PushVal(pubs)
		v.Estack().PushVal(msg)
		require.Error(t, v.Run())
	})
}

func TestCheckSig(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	verifyFunc := ECDSASecp256r1CheckSig
	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	ic := &interop.Context{DAO: dao.NewCached(d)}
	runCase := func(t *testing.T, isErr bool, result interface{}, args ...interface{}) {
		ic.SpawnVM()
		for i := range args {
			ic.VM.Estack().PushVal(args[i])
		}

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = verifyFunc(ic)
		}()

		if isErr {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, 1, ic.VM.Estack().Len())
		require.Equal(t, result, ic.VM.Estack().Pop().Value().(bool))
	}

	tx := transaction.New(netmode.UnitTestNet, []byte{0, 1, 2}, 1)
	msg := tx.GetSignedPart()
	ic.Container = tx

	t.Run("success", func(t *testing.T) {
		sign := priv.Sign(msg)
		runCase(t, false, true, sign, priv.PublicKey().Bytes())
	})

	t.Run("missing argument", func(t *testing.T) {
		runCase(t, true, false)
		sign := priv.Sign(msg)
		runCase(t, true, false, sign)
	})

	t.Run("invalid signature", func(t *testing.T) {
		sign := priv.Sign(msg)
		sign[0] = ^sign[0]
		runCase(t, false, false, sign, priv.PublicKey().Bytes())
	})

	t.Run("invalid public key", func(t *testing.T) {
		sign := priv.Sign(msg)
		pub := priv.PublicKey().Bytes()
		pub[0] = 0xFF // invalid prefix
		runCase(t, true, false, sign, pub)
	})
}
