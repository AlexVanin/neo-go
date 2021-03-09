package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func (s *service) signAndSend(r *state.MPTRoot) error {
	if !s.MainCfg.Enabled {
		return nil
	}

	acc := s.getAccount()
	if acc == nil {
		return nil
	}

	sig := acc.PrivateKey().SignHash(r.GetSignedHash())
	incRoot := s.getIncompleteRoot(r.Index)
	incRoot.root = r
	incRoot.addSignature(acc.PrivateKey().PublicKey(), sig)
	incRoot.reverify()

	s.accMtx.RLock()
	myIndex := s.myIndex
	s.accMtx.RUnlock()
	msg := NewMessage(VoteT, &Vote{
		ValidatorIndex: int32(myIndex),
		Height:         r.Index,
		Signature:      sig,
	})

	w := io.NewBufBinWriter()
	msg.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	s.getRelayCallback()(&payload.Extensible{
		Network:         s.Network,
		ValidBlockStart: r.Index,
		ValidBlockEnd:   r.Index + transaction.MaxValidUntilBlockIncrement,
		Sender:          s.getAccount().PrivateKey().GetScriptHash(),
		Data:            w.Bytes(),
	})
	return nil
}

func (s *service) getAccount() *wallet.Account {
	s.accMtx.RLock()
	defer s.accMtx.RUnlock()
	return s.acc
}
