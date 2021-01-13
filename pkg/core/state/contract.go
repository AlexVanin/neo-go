package state

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Contract holds information about a smart contract in the NEO blockchain.
type Contract struct {
	ID            int32             `json:"id"`
	UpdateCounter uint16            `json:"updatecounter"`
	Hash          util.Uint160      `json:"hash"`
	NEF           nef.File          `json:"nef"`
	Manifest      manifest.Manifest `json:"manifest"`
}

// DecodeBinary implements Serializable interface.
func (c *Contract) DecodeBinary(r *io.BinReader) {
	si := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil {
		return
	}
	r.Err = c.FromStackItem(si)
}

// EncodeBinary implements Serializable interface.
func (c *Contract) EncodeBinary(w *io.BinWriter) {
	si, err := c.ToStackItem()
	if err != nil {
		w.Err = err
		return
	}
	stackitem.EncodeBinaryStackItem(si, w)
}

// ToStackItem converts state.Contract to stackitem.Item
func (c *Contract) ToStackItem() (stackitem.Item, error) {
	manifest, err := json.Marshal(c.Manifest)
	if err != nil {
		return nil, err
	}
	rawNef, err := c.NEF.Bytes()
	if err != nil {
		return nil, err
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.Make(c.ID),
		stackitem.Make(c.UpdateCounter),
		stackitem.NewByteArray(c.Hash.BytesBE()),
		stackitem.NewByteArray(rawNef),
		stackitem.NewByteArray(manifest),
	}), nil
}

// FromStackItem fills Contract's data from given stack itemized contract
// representation.
func (c *Contract) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	bi, ok := arr[0].Value().(*big.Int)
	if !ok {
		return errors.New("ID is not an integer")
	}
	if !bi.IsInt64() || bi.Int64() > math.MaxInt32 || bi.Int64() < math.MinInt32 {
		return errors.New("ID not in int32 range")
	}
	c.ID = int32(bi.Int64())
	bi, ok = arr[1].Value().(*big.Int)
	if !ok {
		return errors.New("UpdateCounter is not an integer")
	}
	if !bi.IsInt64() || bi.Int64() > math.MaxUint16 || bi.Int64() < 0 {
		return errors.New("UpdateCounter not in uint16 range")
	}
	c.UpdateCounter = uint16(bi.Int64())
	bytes, err := arr[2].TryBytes()
	if err != nil {
		return err
	}
	c.Hash, err = util.Uint160DecodeBytesBE(bytes)
	if err != nil {
		return err
	}
	bytes, err = arr[3].TryBytes()
	if err != nil {
		return err
	}
	c.NEF, err = nef.FileFromBytes(bytes)
	if err != nil {
		return err
	}
	bytes, err = arr[4].TryBytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &c.Manifest)
}

// CreateContractHash creates deployed contract hash from transaction sender
// and contract script.
func CreateContractHash(sender util.Uint160, script []byte) util.Uint160 {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	emit.Bytes(w.BinWriter, sender.BytesBE())
	emit.Bytes(w.BinWriter, script)
	if w.Err != nil {
		panic(w.Err)
	}
	return hash.Hash160(w.Bytes())
}

// CreateNativeContractHash returns script and hash for the native contract.
func CreateNativeContractHash(name string) ([]byte, util.Uint160) {
	w := io.NewBufBinWriter()
	emit.String(w.BinWriter, name)
	emit.Syscall(w.BinWriter, interopnames.SystemContractCallNative)
	if w.Err != nil {
		panic(w.Err)
	}
	script := w.Bytes()
	return script, CreateContractHash(util.Uint160{}, script)
}
