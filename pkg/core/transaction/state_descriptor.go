package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// DescStateType represents the type of StateDescriptor.
type DescStateType uint8

// Valid DescStateType constants.
const (
	Account   DescStateType = 0x40
	Validator DescStateType = 0x48
)

// StateDescriptor ..
type StateDescriptor struct {
	Type  DescStateType
	Key   []byte
	Value []byte
	Field string
}

// DecodeBinary implements the Payload interface.
func (s *StateDescriptor) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	br.ReadLE(&s.Type)

	s.Key = br.ReadBytes()
	s.Value = br.ReadBytes()
	s.Field = br.ReadString()

	return br.Err
}

// EncodeBinary implements the Payload interface.
func (s *StateDescriptor) EncodeBinary(w io.Writer) error {
	return nil
}

func (s *StateDescriptor) Size() int {
	return 1 + util.GetVarSize(s.Key) + util.GetVarSize(s.Value) + util.GetVarSize(s.Field)
}
