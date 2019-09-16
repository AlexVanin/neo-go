package core

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const feeMode = 0x0

// Assets is mapping between AssetID and the AssetState.
type Assets map[util.Uint256]*AssetState

func (a Assets) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range a {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STAsset, hash.Bytes())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

// AssetState represents the state of an NEO registered Asset.
type AssetState struct {
	ID         util.Uint256
	AssetType  transaction.AssetType
	Name       string
	Amount     util.Fixed8
	Available  util.Fixed8
	Precision  uint8
	FeeMode    uint8
	FeeAddress util.Uint160
	Owner      *keys.PublicKey
	Admin      util.Uint160
	Issuer     util.Uint160
	Expiration uint32
	IsFrozen   bool
}

// DecodeBinary implements the Payload interface.
func (a *AssetState) DecodeBinary(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&a.ID)
	br.ReadLE(&a.AssetType)

	a.Name = br.ReadString()

	br.ReadLE(&a.Amount)
	br.ReadLE(&a.Available)
	br.ReadLE(&a.Precision)
	br.ReadLE(&a.FeeMode)
	br.ReadLE(&a.FeeAddress)

	if br.Err != nil {
		return br.Err
	}
	a.Owner = &keys.PublicKey{}
	if err := a.Owner.DecodeBinary(r); err != nil {
		return err
	}
	br.ReadLE(&a.Admin)
	br.ReadLE(&a.Issuer)
	br.ReadLE(&a.Expiration)
	br.ReadLE(&a.IsFrozen)

	return br.Err
}

// EncodeBinary implements the Payload interface.
func (a *AssetState) EncodeBinary(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(a.ID)
	bw.WriteLE(a.AssetType)
	bw.WriteString(a.Name)
	bw.WriteLE(a.Amount)
	bw.WriteLE(a.Available)
	bw.WriteLE(a.Precision)
	bw.WriteLE(a.FeeMode)
	bw.WriteLE(a.FeeAddress)

	if bw.Err != nil {
		return bw.Err
	}
	if err := a.Owner.EncodeBinary(w); err != nil {
		return err
	}
	bw.WriteLE(a.Admin)
	bw.WriteLE(a.Issuer)
	bw.WriteLE(a.Expiration)
	bw.WriteLE(a.IsFrozen)
	return bw.Err
}

// GetName returns the asset name based on its type.
func (a *AssetState) GetName() string {

	if a.AssetType == transaction.GoverningToken {
		return "NEO"
	} else if a.AssetType == transaction.UtilityToken {
		return "NEOGas"
	}

	return a.Name
}
