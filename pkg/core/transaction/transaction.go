package transaction

import (
	"encoding/json"
	"errors"
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxTransactionSize is the upper limit size in bytes that a transaction can reach. It is
	// set to be 102400.
	MaxTransactionSize = 102400
	// MaxValidUntilBlockIncrement is the upper increment size of blockhain height in blocs after
	// exceeding that a transaction should fail validation. It is set to be 2102400.
	MaxValidUntilBlockIncrement = 2102400
	// MaxCosigners is maximum number of cosigners that can be contained within a transaction.
	// It is set to be 16.
	MaxCosigners = 16
)

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The trading version which is currently 0.
	Version uint8

	// Random number to avoid hash collision.
	Nonce uint32

	// Address signed the transaction.
	Sender util.Uint160

	// Fee to be burned.
	SystemFee int64

	// Fee to be distributed to consensus nodes.
	NetworkFee int64

	// Maximum blockchain height exceeding which
	// transaction should fail verification.
	ValidUntilBlock uint32

	// Code to run in NeoVM for this transaction.
	Script []byte

	// Transaction attributes.
	Attributes []Attribute

	// Transaction cosigners (not include Sender).
	Cosigners []Cosigner

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []Witness

	// Network magic number. This one actually is not a part of the
	// wire-representation of Transaction, but it's absolutely necessary
	// for correct signing/verification.
	Network netmode.Magic

	// Hash of the transaction (double SHA256).
	hash util.Uint256

	// Hash of the transaction used to verify it (single SHA256).
	verificationHash util.Uint256

	// Trimmed indicates this is a transaction from trimmed
	// data.
	Trimmed bool
}

// NewTrimmedTX returns a trimmed transaction with only its hash
// and Trimmed to true.
func NewTrimmedTX(hash util.Uint256) *Transaction {
	return &Transaction{
		hash:    hash,
		Trimmed: true,
	}
}

// New returns a new transaction to execute given script and pay given system
// fee.
func New(network netmode.Magic, script []byte, gas int64) *Transaction {
	return &Transaction{
		Version:    0,
		Nonce:      rand.Uint32(),
		Script:     script,
		SystemFee:  gas,
		Attributes: []Attribute{},
		Cosigners:  []Cosigner{},
		Scripts:    []Witness{},
		Network:    network,
	}
}

// Hash returns the hash of the transaction.
func (t *Transaction) Hash() util.Uint256 {
	if t.hash.Equals(util.Uint256{}) {
		if t.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return t.hash
}

// VerificationHash returns the hash of the transaction used to verify it.
func (t *Transaction) VerificationHash() util.Uint256 {
	if t.verificationHash.Equals(util.Uint256{}) {
		if t.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return t.verificationHash
}

// decodeHashableFields decodes the fields that are used for signing the
// transaction, which are all fields except the scripts.
func (t *Transaction) decodeHashableFields(br *io.BinReader) {
	t.Version = uint8(br.ReadB())
	if t.Version > 0 {
		br.Err = errors.New("only version 0 is supported")
		return
	}
	t.Nonce = br.ReadU32LE()
	t.Sender.DecodeBinary(br)
	t.SystemFee = int64(br.ReadU64LE())
	if t.SystemFee < 0 {
		br.Err = errors.New("negative system fee")
		return
	}
	t.NetworkFee = int64(br.ReadU64LE())
	if t.NetworkFee < 0 {
		br.Err = errors.New("negative network fee")
		return
	}
	if t.NetworkFee+t.SystemFee < t.SystemFee {
		br.Err = errors.New("too big fees: int 64 overflow")
		return
	}
	t.ValidUntilBlock = br.ReadU32LE()

	br.ReadArray(&t.Attributes)

	br.ReadArray(&t.Cosigners, MaxCosigners)
	for i := 0; i < len(t.Cosigners); i++ {
		for j := i + 1; j < len(t.Cosigners); j++ {
			if t.Cosigners[i].Account.Equals(t.Cosigners[j].Account) {
				br.Err = errors.New("transaction cosigners should be unique")
				return
			}
		}
	}

	t.Script = br.ReadVarBytes()
	if br.Err == nil && len(t.Script) == 0 {
		br.Err = errors.New("no script")
		return
	}
}

// DecodeBinary implements Serializable interface.
func (t *Transaction) DecodeBinary(br *io.BinReader) {
	t.decodeHashableFields(br)
	if br.Err != nil {
		return
	}
	br.ReadArray(&t.Scripts)

	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	if br.Err == nil {
		br.Err = t.createHash()
	}
}

// EncodeBinary implements Serializable interface.
func (t *Transaction) EncodeBinary(bw *io.BinWriter) {
	t.encodeHashableFields(bw)
	bw.WriteArray(t.Scripts)
}

// encodeHashableFields encodes the fields that are not used for
// signing the transaction, which are all fields except the scripts.
func (t *Transaction) encodeHashableFields(bw *io.BinWriter) {
	if len(t.Script) == 0 {
		bw.Err = errors.New("transaction has no script")
		return
	}
	bw.WriteB(byte(t.Version))
	bw.WriteU32LE(t.Nonce)
	t.Sender.EncodeBinary(bw)
	bw.WriteU64LE(uint64(t.SystemFee))
	bw.WriteU64LE(uint64(t.NetworkFee))
	bw.WriteU32LE(t.ValidUntilBlock)

	// Attributes
	bw.WriteArray(t.Attributes)

	// Cosigners
	bw.WriteArray(t.Cosigners)

	bw.WriteVarBytes(t.Script)
}

// createHash creates the hash of the transaction.
func (t *Transaction) createHash() error {
	b := t.GetSignedPart()
	if b == nil {
		return errors.New("failed to serialize hashable data")
	}
	t.updateHashes(b)
	return nil
}

// updateHashes updates Transaction's hashes based on the given buffer which should
// be a signable data slice.
func (t *Transaction) updateHashes(b []byte) {
	t.verificationHash = hash.Sha256(b)
	t.hash = hash.Sha256(t.verificationHash.BytesBE())
}

// GetSignedPart returns a part of the transaction which must be signed.
func (t *Transaction) GetSignedPart() []byte {
	buf := io.NewBufBinWriter()
	buf.WriteU32LE(uint32(t.Network))
	t.encodeHashableFields(buf.BinWriter)
	if buf.Err != nil {
		return nil
	}
	return buf.Bytes()
}

// DecodeSignedPart decodes a part of transaction from GetSignedPart data.
func (t *Transaction) DecodeSignedPart(buf []byte) error {
	r := io.NewBinReaderFromBuf(buf)
	t.Network = netmode.Magic(r.ReadU32LE())
	t.decodeHashableFields(r)
	if r.Err != nil {
		return r.Err
	}
	// Ensure all the data was read.
	_ = r.ReadB()
	if r.Err == nil {
		return errors.New("additional data after the signed part")
	}
	t.Scripts = make([]Witness, 0)
	t.updateHashes(buf)
	return nil
}

// Bytes converts the transaction to []byte
func (t *Transaction) Bytes() []byte {
	buf := io.NewBufBinWriter()
	t.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil
	}
	return buf.Bytes()
}

// NewTransactionFromBytes decodes byte array into *Transaction
func NewTransactionFromBytes(network netmode.Magic, b []byte) (*Transaction, error) {
	tx := &Transaction{Network: network}
	r := io.NewBinReaderFromBuf(b)
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return tx, nil
}

// FeePerByte returns NetworkFee of the transaction divided by
// its size
func (t *Transaction) FeePerByte() int64 {
	return t.NetworkFee / int64(io.GetVarSize(t))
}

// transactionJSON is a wrapper for Transaction and
// used for correct marhalling of transaction.Data
type transactionJSON struct {
	TxID            util.Uint256 `json:"hash"`
	Size            int          `json:"size"`
	Version         uint8        `json:"version"`
	Nonce           uint32       `json:"nonce"`
	Sender          string       `json:"sender"`
	SystemFee       int64        `json:"sys_fee,string"`
	NetworkFee      int64        `json:"net_fee,string"`
	ValidUntilBlock uint32       `json:"valid_until_block"`
	Attributes      []Attribute  `json:"attributes"`
	Cosigners       []Cosigner   `json:"cosigners"`
	Script          []byte       `json:"script"`
	Scripts         []Witness    `json:"scripts"`
}

// MarshalJSON implements json.Marshaler interface.
func (t *Transaction) MarshalJSON() ([]byte, error) {
	tx := transactionJSON{
		TxID:            t.Hash(),
		Size:            io.GetVarSize(t),
		Version:         t.Version,
		Nonce:           t.Nonce,
		Sender:          address.Uint160ToString(t.Sender),
		ValidUntilBlock: t.ValidUntilBlock,
		Attributes:      t.Attributes,
		Cosigners:       t.Cosigners,
		Script:          t.Script,
		Scripts:         t.Scripts,
		SystemFee:       t.SystemFee,
		NetworkFee:      t.NetworkFee,
	}
	return json.Marshal(tx)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	tx := new(transactionJSON)
	if err := json.Unmarshal(data, tx); err != nil {
		return err
	}
	t.Version = tx.Version
	t.Nonce = tx.Nonce
	t.ValidUntilBlock = tx.ValidUntilBlock
	t.Attributes = tx.Attributes
	t.Cosigners = tx.Cosigners
	t.Scripts = tx.Scripts
	t.SystemFee = tx.SystemFee
	t.NetworkFee = tx.NetworkFee
	sender, err := address.StringToUint160(tx.Sender)
	if err != nil {
		return errors.New("cannot unmarshal tx: bad sender")
	}
	t.Sender = sender
	t.Script = tx.Script
	if t.Hash() != tx.TxID {
		return errors.New("txid doesn't match transaction hash")
	}

	return nil
}
