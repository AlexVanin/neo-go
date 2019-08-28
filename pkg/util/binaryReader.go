package util

import (
	"encoding/binary"
	"io"
)

//BinReader is a convenient wrapper around a io.Reader and err object
// Used to simplify error handling when reading into a struct with many fields
type BinReader struct {
	R   io.Reader
	Err error
}

// ReadLE reads from the underlying io.Reader
// into the interface v in little-endian format
func (r *BinReader) ReadLE(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.R, binary.LittleEndian, v)
}

// ReadBE reads from the underlying io.Reader
// into the interface v in big-endian format
func (r *BinReader) ReadBE(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.R, binary.BigEndian, v)
}

// ReadVarUint reads a variable-length-encoded integer from the
// underlying reader
func (r *BinReader) ReadVarUint() uint64 {
	var b uint8
	r.Err = binary.Read(r.R, binary.LittleEndian, &b)

	if b == 0xfd {
		var v uint16
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xfe {
		var v uint32
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xff {
		var v uint64
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return v
	}

	return uint64(b)
}

// ReadBytes reads the next set of bytes from the underlying reader.
// ReadVarUInt() is used to determine how large that slice is
func (r *BinReader) ReadBytes() []byte {
	n := r.ReadVarUint()
	b := make([]byte, n)
	r.ReadLE(b)
	return b
}

// ReadString calls ReadBytes and casts the results as a string
func (r *BinReader) ReadString() string {
	b := r.ReadBytes()
	return string(b)
}
