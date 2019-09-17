package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// SpentCoins is mapping between transactions and their spent
// coin state.
type SpentCoins map[util.Uint256]*SpentCoinState

func (s SpentCoins) getAndUpdate(store storage.Store, hash util.Uint256) (*SpentCoinState, error) {
	if spent, ok := s[hash]; ok {
		return spent, nil
	}

	spent := &SpentCoinState{}
	key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesReverse())
	if b, err := store.Get(key); err == nil {
		r := io.NewBinReaderFromBuf(b)
		spent.DecodeBinary(r)
		if r.Err != nil {
			return nil, fmt.Errorf("failed to decode (UnspentCoinState): %s", r.Err)
		}
	} else {
		spent = &SpentCoinState{
			items: make(map[uint16]uint32),
		}
	}

	s[hash] = spent
	return spent, nil
}

func (s SpentCoins) commit(b storage.Batch) error {
	buf := io.NewBufBinWriter()
	for hash, state := range s {
		state.EncodeBinary(buf.BinWriter)
		if buf.Err != nil {
			return buf.Err
		}
		key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesReverse())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

// SpentCoinState represents the state of a spent coin.
type SpentCoinState struct {
	txHash   util.Uint256
	txHeight uint32

	// A mapping between the index of the prevIndex and block height.
	items map[uint16]uint32
}

// NewSpentCoinState returns a new SpentCoinState object.
func NewSpentCoinState(hash util.Uint256, height uint32) *SpentCoinState {
	return &SpentCoinState{
		txHash:   hash,
		txHeight: height,
		items:    make(map[uint16]uint32),
	}
}

// DecodeBinary implements Serializable interface.
func (s *SpentCoinState) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&s.txHash)
	br.ReadLE(&s.txHeight)

	s.items = make(map[uint16]uint32)
	lenItems := br.ReadVarUint()
	for i := 0; i < int(lenItems); i++ {
		var (
			key   uint16
			value uint32
		)
		br.ReadLE(&key)
		br.ReadLE(&value)
		s.items[key] = value
	}
}

// EncodeBinary implements Serializable interface.
func (s *SpentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(s.txHash)
	bw.WriteLE(s.txHeight)
	bw.WriteVarUint(uint64(len(s.items)))
	for k, v := range s.items {
		bw.WriteLE(k)
		bw.WriteLE(v)
	}
}
