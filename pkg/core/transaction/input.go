package transaction

import (
	"sort"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Input represents a Transaction input (CoinReference).
type Input struct {
	// The hash of the previous transaction.
	PrevHash util.Uint256 `json:"txid"`

	// The index of the previous transaction.
	PrevIndex uint16 `json:"vout"`
}

// DecodeBinary implements Serializable interface.
func (in *Input) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(in.PrevHash[:])
	in.PrevIndex = br.ReadU16LE()
}

// EncodeBinary implements Serializable interface.
func (in *Input) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(in.PrevHash[:])
	bw.WriteU16LE(in.PrevIndex)
}

// GroupInputsByPrevHash groups all TX inputs by their previous hash into
// several slices (which actually are subslices of one new slice with pointers).
// Each of these slices contains at least one element.
func GroupInputsByPrevHash(ins []Input) [][]*Input {
	if len(ins) == 0 {
		return nil
	}

	ptrs := make([]*Input, len(ins))
	for i := range ins {
		ptrs[i] = &ins[i]
	}
	sort.Slice(ptrs, func(i, j int) bool {
		return ptrs[i].PrevHash.CompareTo(ptrs[j].PrevHash) < 0
	})

	var first int
	res := make([][]*Input, 0)
	currentHash := ptrs[0].PrevHash

	for i := range ptrs {
		if !currentHash.Equals(ptrs[i].PrevHash) {
			res = append(res, ptrs[first:i])
			first = i
			currentHash = ptrs[i].PrevHash
		}
	}
	res = append(res, ptrs[first:])
	return res
}
