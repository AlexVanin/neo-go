package core

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func (bc *Blockchain) setNodesByRole(t *testing.T, ok bool, r native.Role, nodes keys.PublicKeys) {
	w := io.NewBufBinWriter()
	for _, pub := range nodes {
		emit.Bytes(w.BinWriter, pub.Bytes())
	}
	emit.Int(w.BinWriter, int64(len(nodes)))
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.Int(w.BinWriter, int64(r))
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, bc.contracts.Designate.Hash, "designateAsRole", callflag.All)
	require.NoError(t, w.Err)
	tx := transaction.New(netmode.UnitTestNet, w.Bytes(), 0)
	tx.NetworkFee = 10_000_000
	tx.SystemFee = 10_000_000
	tx.ValidUntilBlock = 100
	tx.Signers = []transaction.Signer{
		{
			Account: testchain.MultisigScriptHash(),
			Scopes:  transaction.None,
		},
		{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
	}
	require.NoError(t, testchain.SignTx(bc, tx))
	tx.Scripts = append(tx.Scripts, transaction.Witness{
		InvocationScript:   testchain.SignCommittee(tx.GetSignedPart()),
		VerificationScript: testchain.CommitteeVerificationScript(),
	})
	require.NoError(t, bc.AddBlock(bc.newBlock(tx)))

	aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	if ok {
		require.Equal(t, vm.HaltState, aer[0].VMState)
	} else {
		require.Equal(t, vm.FaultState, aer[0].VMState)
	}
}

func (bc *Blockchain) getNodesByRole(t *testing.T, ok bool, r native.Role, index uint32, resLen int) {
	res, err := invokeContractMethod(bc, 10_000_000, bc.contracts.Designate.Hash, "getDesignatedByRole", int64(r), int64(index))
	require.NoError(t, err)
	if ok {
		require.Equal(t, vm.HaltState, res.VMState)
		require.Equal(t, 1, len(res.Stack))
		arrItem := res.Stack[0]
		require.Equal(t, stackitem.ArrayT, arrItem.Type())
		arr := arrItem.(*stackitem.Array)
		require.Equal(t, resLen, arr.Len())
	} else {
		checkFAULTState(t, res)
	}
}

func TestDesignate_DesignateAsRoleTx(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubs := keys.PublicKeys{priv.PublicKey()}

	bc.setNodesByRole(t, false, 0xFF, pubs)
	bc.setNodesByRole(t, true, native.RoleOracle, pubs)
	index := bc.BlockHeight() + 1
	bc.getNodesByRole(t, false, 0xFF, 0, 0)
	bc.getNodesByRole(t, false, native.RoleOracle, 100500, 0)
	bc.getNodesByRole(t, true, native.RoleOracle, 0, 0)     // returns an empty list
	bc.getNodesByRole(t, true, native.RoleOracle, index, 1) // returns pubs
}

func TestDesignate_DesignateAsRole(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	des := bc.contracts.Designate
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	bl := block.New(netmode.UnitTestNet, bc.config.StateRootInHeader)
	bl.Index = bc.BlockHeight() + 1
	ic := bc.newInteropContext(trigger.OnPersist, bc.dao, bl, tx)
	ic.SpawnVM()
	ic.VM.LoadScript([]byte{byte(opcode.RET)})

	pubs, index, err := des.GetDesignatedByRole(bc.dao, 0xFF, 255)
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	pubs, index, err = des.GetDesignatedByRole(bc.dao, native.RoleOracle, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{})
	require.True(t, errors.Is(err, native.ErrEmptyNodeList), "got: %v", err)

	err = des.DesignateAsRole(ic, native.RoleOracle, make(keys.PublicKeys, 32+1))
	require.True(t, errors.Is(err, native.ErrLargeNodeList), "got: %v", err)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	err = des.DesignateAsRole(ic, 0xFF, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidWitness), "got: %v", err)

	setSigner(tx, testchain.CommitteeScriptHash())
	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{pub})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleOracle, bl.Index+1)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)
	require.Equal(t, bl.Index+1, index)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleStateValidator, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	// Set StateValidator role.
	_, err = keys.NewPrivateKey()
	require.NoError(t, err)
	pub1 := priv.PublicKey()
	err = des.DesignateAsRole(ic, native.RoleStateValidator, keys.PublicKeys{pub1})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleOracle, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)
	require.Equal(t, bl.Index+1, index)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleStateValidator, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub1}, pubs)
	require.Equal(t, bl.Index+1, index)

	// Set P2PNotary role.
	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleP2PNotary, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	err = des.DesignateAsRole(ic, native.RoleP2PNotary, keys.PublicKeys{pub1})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, native.RoleP2PNotary, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub1}, pubs)
	require.Equal(t, bl.Index+1, index)
}
