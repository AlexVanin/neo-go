package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestDecodeEncodeUnspentCoinState(t *testing.T) {
	unspent := &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateSpent,
			CoinStateSpent,
			CoinStateSpent,
			CoinStateConfirmed,
		},
	}

	buf := io.NewBufBinWriter()
	assert.Nil(t, unspent.EncodeBinary(buf.BinWriter))
	unspentDecode := &UnspentCoinState{}
	assert.Nil(t, unspentDecode.DecodeBinary(io.NewBinReaderFromBuf(buf.Bytes())))
}

func TestCommitUnspentCoins(t *testing.T) {
	var (
		store        = storage.NewMemoryStore()
		batch        = store.Batch()
		unspentCoins = make(UnspentCoins)
	)

	txA := randomUint256()
	txB := randomUint256()
	txC := randomUint256()

	unspentCoins[txA] = &UnspentCoinState{
		states: []CoinState{CoinStateConfirmed},
	}
	unspentCoins[txB] = &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateConfirmed,
		},
	}
	unspentCoins[txC] = &UnspentCoinState{
		states: []CoinState{
			CoinStateConfirmed,
			CoinStateConfirmed,
			CoinStateConfirmed,
		},
	}

	assert.Nil(t, unspentCoins.commit(batch))
	assert.Nil(t, store.PutBatch(batch))
}
