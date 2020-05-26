package network

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

//go:generate stringer -type=CommandType

const (
	// PayloadMaxSize is maximum payload size in decompressed form.
	PayloadMaxSize = 0x02000000
	// CompressionMinSize is the lower bound to apply compression.
	CompressionMinSize = 1024
)

// Message is the complete message send between nodes.
type Message struct {
	// Flags that represents whether a message is compressed.
	// 0 for None, 1 for Compressed.
	Flags MessageFlag
	// Command is byte command code.
	Command CommandType

	// Payload send with the message.
	Payload payload.Payload

	// Compressed message payload.
	compressedPayload []byte
}

// MessageFlag represents compression level of message payload
type MessageFlag byte

// Possible message flags
const (
	None       MessageFlag = 0
	Compressed MessageFlag = 1 << iota
)

// CommandType represents the type of a message command.
type CommandType byte

// Valid protocol commands used to send between nodes.
const (
	// handshaking
	CMDVersion CommandType = 0x00
	CMDVerack  CommandType = 0x01

	// connectivity
	CMDGetAddr CommandType = 0x10
	CMDAddr    CommandType = 0x11
	CMDPing    CommandType = 0x18
	CMDPong    CommandType = 0x19

	// synchronization
	CMDGetHeaders   CommandType = 0x20
	CMDHeaders      CommandType = 0x21
	CMDGetBlocks    CommandType = 0x24
	CMDMempool      CommandType = 0x25
	CMDInv          CommandType = 0x27
	CMDGetData      CommandType = 0x28
	CMDGetBlockData CommandType = 0x29
	CMDUnknown      CommandType = 0x2a
	CMDTX           CommandType = 0x2b
	CMDBlock        CommandType = 0x2c
	CMDConsensus    CommandType = 0x2d
	CMDReject       CommandType = 0x2f

	// SPV protocol
	CMDFilterLoad  CommandType = 0x30
	CMDFilterAdd   CommandType = 0x31
	CMDFilterClear CommandType = 0x32
	CMDMerkleBlock CommandType = 0x38

	// others
	CMDAlert CommandType = 0x40
)

// NewMessage returns a new message with the given payload.
func NewMessage(cmd CommandType, p payload.Payload) *Message {
	return &Message{
		Command: cmd,
		Payload: p,
		Flags:   None,
	}
}

// Decode decodes a Message from the given reader.
func (m *Message) Decode(br *io.BinReader) error {
	m.Flags = MessageFlag(br.ReadB())
	m.Command = CommandType(br.ReadB())
	l := br.ReadVarUint()
	// check the length first in order not to allocate memory
	// for an empty compressed payload
	if l == 0 {
		m.Payload = payload.NewNullPayload()
		return nil
	}
	m.compressedPayload = make([]byte, l)
	br.ReadBytes(m.compressedPayload)
	if br.Err != nil {
		return br.Err
	}
	if len(m.compressedPayload) > PayloadMaxSize {
		return errors.New("invalid payload size")
	}
	return m.decodePayload()
}

func (m *Message) decodePayload() error {
	buf := m.compressedPayload
	// try decompression
	if m.Flags&Compressed != 0 {
		d, err := decompress(m.compressedPayload)
		if err != nil {
			return err
		}
		buf = d
	}

	r := io.NewBinReaderFromBuf(buf)
	var p payload.Payload
	switch m.Command {
	case CMDVersion:
		p = &payload.Version{}
	case CMDInv, CMDGetData:
		p = &payload.Inventory{}
	case CMDAddr:
		p = &payload.AddressList{}
	case CMDBlock:
		p = &block.Block{}
	case CMDConsensus:
		p = &consensus.Payload{}
	case CMDGetBlocks:
		fallthrough
	case CMDGetHeaders:
		p = &payload.GetBlocks{}
	case CMDGetBlockData:
		p = &payload.GetBlockData{}
	case CMDHeaders:
		p = &payload.Headers{}
	case CMDTX:
		p = &transaction.Transaction{}
	case CMDMerkleBlock:
		p = &payload.MerkleBlock{}
	case CMDPing, CMDPong:
		p = &payload.Ping{}
	default:
		return fmt.Errorf("can't decode command %s", m.Command.String())
	}
	p.DecodeBinary(r)
	if r.Err == nil || r.Err == payload.ErrTooManyHeaders {
		m.Payload = p
	}

	return r.Err
}

// Encode encodes a Message to any given BinWriter.
func (m *Message) Encode(br *io.BinWriter) error {
	if err := m.tryCompressPayload(); err != nil {
		return err
	}
	br.WriteB(byte(m.Flags))
	br.WriteB(byte(m.Command))
	if m.compressedPayload != nil {
		br.WriteVarBytes(m.compressedPayload)
	} else {
		br.WriteB(0)
	}
	return br.Err
}

// Bytes serializes a Message into the new allocated buffer and returns it.
func (m *Message) Bytes() ([]byte, error) {
	w := io.NewBufBinWriter()
	if err := m.Encode(w.BinWriter); err != nil {
		return nil, err
	}
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

// tryCompressPayload sets message's compressed payload to serialized
// payload and compresses it in case if its size exceeds CompressionMinSize
func (m *Message) tryCompressPayload() error {
	if m.Payload == nil {
		return nil
	}
	buf := io.NewBufBinWriter()
	m.Payload.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	compressedPayload := buf.Bytes()
	if m.Flags&Compressed == 0 {
		switch m.Payload.(type) {
		case *payload.Headers, *payload.MerkleBlock, *payload.NullPayload:
			break
		default:
			size := len(compressedPayload)
			// try compression
			if size > CompressionMinSize {
				c, err := compress(compressedPayload)
				if err == nil {
					compressedPayload = c
					m.Flags |= Compressed
				} else {
					return err
				}
			}
		}
	}
	m.compressedPayload = compressedPayload
	return nil
}
