package state

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeSpentCoin(t *testing.T) {
	spent := &SpentCoin{
		TxHeight: 1001,
		Items: map[uint16]uint32{
			1: 3,
			2: 8,
			4: 100,
		},
	}

	buf := io.NewBufBinWriter()
	spent.EncodeBinary(buf.BinWriter)
	assert.Nil(t, buf.Err)
	spentDecode := new(SpentCoin)
	r := io.NewBinReaderFromBuf(buf.Bytes())
	spentDecode.DecodeBinary(r)
	assert.Nil(t, r.Err)
	assert.Equal(t, spent, spentDecode)
}
