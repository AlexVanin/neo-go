package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

// testScriptGetter is an auxilliary struct to pass CheckWitness checks.
type testScriptGetter struct {
	h util.Uint160
}

func (t testScriptGetter) GetCallingScriptHash() util.Uint160 { return t.h }
func (t testScriptGetter) GetEntryScriptHash() util.Uint160   { return t.h }
func (t testScriptGetter) GetCurrentScriptHash() util.Uint160 { return t.h }

func setSigner(tx *transaction.Transaction, h util.Uint160) {
	tx.Signers = []transaction.Signer{{
		Account: h,
		Scopes:  transaction.Global,
	}}
}

func TestNEO_Vote(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	neo := bc.contracts.NEO
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)

	pubs, err := neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, bc.GetStandByValidators(), pubs)

	sz := testchain.Size()

	candidates := make(keys.PublicKeys, sz)
	for i := 0; i < sz; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		candidates[i] = priv.PublicKey()
		if i > 0 {
			require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[i]))
		}
	}

	for i := 0; i < sz; i++ {
		to := testchain.PrivateKeyByID(i).GetScriptHash()
		ic.ScriptGetter = testScriptGetter{testchain.MultisigScriptHash()}
		require.NoError(t, neo.TransferInternal(ic, testchain.MultisigScriptHash(), to, big.NewInt(int64(sz-i)*10000000)))
	}

	for i := 1; i < sz; i++ {
		h := testchain.PrivateKeyByID(i).GetScriptHash()
		setSigner(tx, h)
		ic.ScriptGetter = testScriptGetter{h}
		require.NoError(t, neo.VoteInternal(ic, h, candidates[i]))
	}

	// First 3 validators must be the ones we have voted for.
	pubs, err = neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	for i := 1; i < sz; i++ {
		require.Equal(t, pubs[i-1], candidates[i])
	}

	var ok bool
	for _, p := range bc.GetStandByValidators() {
		if pubs[sz-1].Equal(p) {
			ok = true
			break
		}
	}
	require.True(t, ok, "last validator must be stand by")

	// Register and give some value to the last validator.
	require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[0]))
	h := testchain.PrivateKeyByID(0).GetScriptHash()
	setSigner(tx, h)
	ic.ScriptGetter = testScriptGetter{h}
	require.NoError(t, neo.VoteInternal(ic, h, candidates[0]))

	pubs, err = neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, candidates, pubs)
}
