package transaction

import (
	"encoding/json"
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Output represents a Transaction output.
type Output struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount util.Fixed8

	// The address of the recipient.
	ScriptHash util.Uint160

	// The position of the Output in slice []Output. This is actually set in NewTransactionOutputRaw
	// and used for diplaying purposes.
	Position int
}

// NewOutput returns a new transaction output.
func NewOutput(assetID util.Uint256, amount util.Fixed8, scriptHash util.Uint160) *Output {
	return &Output{
		AssetID:    assetID,
		Amount:     amount,
		ScriptHash: scriptHash,
	}
}

// DecodeBinary implements the Payload interface.
func (out *Output) DecodeBinary(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&out.AssetID)
	br.ReadLE(&out.Amount)
	br.ReadLE(&out.ScriptHash)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (out *Output) EncodeBinary(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(out.AssetID)
	bw.WriteLE(out.Amount)
	bw.WriteLE(out.ScriptHash)
	return bw.Err
}

// Size returns the size in bytes of the Output
func (out *Output) Size() int {
	return out.AssetID.Size() + out.Amount.Size() + out.ScriptHash.Size()
}

// MarshalJSON implements the Marshaler interface
func (out *Output) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"asset":   out.AssetID,
		"value":   out.Amount,
		"address": crypto.AddressFromUint160(out.ScriptHash),
		"n":       out.Position,
	})
}
