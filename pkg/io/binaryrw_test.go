package io

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mocks io.Reader and io.Writer, always fails to Write() or Read().
type badRW struct{}

func (w *badRW) Write(p []byte) (int, error) {
	return 0, errors.New("it always fails")
}

func (w *badRW) Read(p []byte) (int, error) {
	return w.Write(p)
}

func TestWriteLE(t *testing.T) {
	var (
		val     uint32 = 0xdeadbeef
		readval uint32
		bin     = []byte{0xef, 0xbe, 0xad, 0xde}
	)
	bw := NewBufBinWriter()
	bw.WriteLE(val)
	assert.Nil(t, bw.Err)
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	br.ReadLE(&readval)
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriteBE(t *testing.T) {
	var (
		val     uint32 = 0xdeadbeef
		readval uint32
		bin     = []byte{0xde, 0xad, 0xbe, 0xef}
	)
	bw := NewBufBinWriter()
	bw.WriteBE(val)
	assert.Nil(t, bw.Err)
	wrotebin := bw.Bytes()
	assert.Equal(t, wrotebin, bin)
	br := NewBinReaderFromBuf(bin)
	br.ReadBE(&readval)
	assert.Nil(t, br.Err)
	assert.Equal(t, val, readval)
}

func TestWriterErrHandling(t *testing.T) {
	var badio = &badRW{}
	bw := NewBinWriterFromIO(badio)
	bw.WriteLE(uint32(0))
	assert.NotNil(t, bw.Err)
	// these should work (without panic), preserving the Err
	bw.WriteLE(uint32(0))
	bw.WriteBE(uint32(0))
	bw.WriteVarUint(0)
	bw.WriteBytes([]byte{0x55, 0xaa})
	bw.WriteString("neo")
	assert.NotNil(t, bw.Err)
}

func TestReaderErrHandling(t *testing.T) {
	var (
		i     uint32 = 0xdeadbeef
		iorig        = i
		badio        = &badRW{}
	)
	br := NewBinReaderFromIO(badio)
	br.ReadLE(&i)
	assert.NotNil(t, br.Err)
	// i shouldn't change
	assert.Equal(t, i, iorig)
	// these should work (without panic), preserving the Err
	br.ReadLE(&i)
	br.ReadBE(&i)
	assert.Equal(t, i, iorig)
	val := br.ReadVarUint()
	assert.Equal(t, val, uint64(0))
	b := br.ReadBytes()
	assert.Equal(t, b, []byte{})
	s := br.ReadString()
	assert.Equal(t, s, "")
	assert.NotNil(t, br.Err)
}

func TestBufBinWriterErr(t *testing.T) {
	bw := NewBufBinWriter()
	bw.WriteLE(uint32(0))
	assert.Nil(t, bw.Err)
	// inject error
	bw.Err = errors.New("oopsie")
	res := bw.Bytes()
	assert.NotNil(t, bw.Err)
	assert.Nil(t, res)
}

func TestBufBinWriterReset(t *testing.T) {
	bw := NewBufBinWriter()
	for i := 0; i < 3; i++ {
		bw.WriteLE(uint32(i))
		assert.Nil(t, bw.Err)
		_ = bw.Bytes()
		assert.NotNil(t, bw.Err)
		bw.Reset()
		assert.Nil(t, bw.Err)
	}
}

func TestWriteString(t *testing.T) {
	var (
		str = "teststring"
	)
	bw := NewBufBinWriter()
	bw.WriteString(str)
	assert.Nil(t, bw.Err)
	wrotebin := bw.Bytes()
	// +1 byte for length
	assert.Equal(t, len(wrotebin), len(str)+1)
	br := NewBinReaderFromBuf(wrotebin)
	readstr := br.ReadString()
	assert.Nil(t, br.Err)
	assert.Equal(t, str, readstr)
}

func TestWriteVarUint1(t *testing.T) {
	var (
		val = uint64(1)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 1, len(buf))
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint1000(t *testing.T) {
	var (
		val = uint64(1000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 3, len(buf))
	assert.Equal(t, byte(0xfd), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000(t *testing.T) {
	var (
		val = uint64(100000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 5, len(buf))
	assert.Equal(t, byte(0xfe), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

func TestWriteVarUint100000000000(t *testing.T) {
	var (
		val = uint64(1000000000000)
	)
	bw := NewBufBinWriter()
	bw.WriteVarUint(val)
	assert.Nil(t, bw.Err)
	buf := bw.Bytes()
	assert.Equal(t, 9, len(buf))
	assert.Equal(t, byte(0xff), buf[0])
	br := NewBinReaderFromBuf(buf)
	res := br.ReadVarUint()
	assert.Nil(t, br.Err)
	assert.Equal(t, val, res)
}

type testSerializable uint16

// EncodeBinary implements io.Serializable interface.
func (t testSerializable) EncodeBinary(w *BinWriter) {
	w.WriteLE(t)
}

// DecodeBinary implements io.Serializable interface.
func (t *testSerializable) DecodeBinary(r *BinReader) {
	r.ReadLE(t)
}

func TestBinWriter_WriteArray(t *testing.T) {
	var arr [3]testSerializable
	for i := range arr {
		arr[i] = testSerializable(i)
	}

	expected := []byte{3, 0, 0, 1, 0, 2, 0}

	w := NewBufBinWriter()
	w.WriteArray(arr)
	require.NoError(t, w.Err)
	require.Equal(t, expected, w.Bytes())

	w.Reset()
	w.WriteArray(arr[:])
	require.NoError(t, w.Err)
	require.Equal(t, expected, w.Bytes())

	arrS := make([]Serializable, len(arr))
	for i := range arrS {
		arrS[i] = &arr[i]
	}

	w.Reset()
	w.WriteArray(arr)
	require.NoError(t, w.Err)
	require.Equal(t, expected, w.Bytes())

	w.Reset()
	require.Panics(t, func() { w.WriteArray(1) })

	w.Reset()
	w.Err = errors.New("error")
	w.WriteArray(arr[:])
	require.Error(t, w.Err)
	require.Equal(t, w.Bytes(), []byte(nil))

	w.Reset()
	w.Err = errors.New("error")
	require.Panics(t, func() { w.WriteArray([]int{}) })

	w.Reset()
	w.Err = errors.New("error")
	require.Panics(t, func() { w.WriteArray(make(chan testSerializable)) })
}

func TestBinReader_ReadArray(t *testing.T) {
	data := []byte{3, 0, 0, 1, 0, 2, 0}
	r := NewBinReaderFromBuf(data)
	result := r.ReadArray(testSerializable(0))
	elems := []testSerializable{0, 1, 2}
	require.Equal(t, []*testSerializable{&elems[0], &elems[1], &elems[2]}, result)

	r = NewBinReaderFromBuf(data)
	r.Err = errors.New("error")
	result = r.ReadArray(testSerializable(0))
	require.Error(t, r.Err)
	require.Equal(t, ([]*testSerializable)(nil), result)

	r = NewBinReaderFromBuf([]byte{0})
	result = r.ReadArray(testSerializable(0))
	require.NoError(t, r.Err)
	require.Equal(t, []*testSerializable{}, result)

	r = NewBinReaderFromBuf([]byte{0})
	require.Panics(t, func() { r.ReadArray(0) })
}
