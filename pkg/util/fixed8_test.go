package util

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFixed8(t *testing.T) {
	values := []int64{9000, 100000000, 5, 10945, -42}

	for _, val := range values {
		assert.Equal(t, Fixed8(val*decimals), NewFixed8(val))
		assert.Equal(t, val, NewFixed8(val).Int64Value())
	}
}

func TestFixed8Add(t *testing.T) {
	a := NewFixed8(1)
	b := NewFixed8(2)

	c := a.Add(b)
	expected := int64(3)
	assert.Equal(t, strconv.FormatInt(expected, 10), c.String())
}

func TestFixed8Sub(t *testing.T) {

	a := NewFixed8(42)
	b := NewFixed8(34)

	c := a.Sub(b)
	assert.Equal(t, int64(8), c.Int64Value())
}

func TestFixed8FromFloat(t *testing.T) {
	inputs := []float64{12.98, 23.87654333, 100.654322, 456789.12345665, -3.14159265}

	for _, val := range inputs {
		assert.Equal(t, Fixed8(val*decimals), NewFixed8FromFloat(val))
		assert.Equal(t, val, NewFixed8FromFloat(val).FloatValue())
	}
}

func TestFixed8DecodeString(t *testing.T) {
	// Fixed8DecodeString works correctly with integers
	ivalues := []string{"9000", "100000000", "5", "10945", "20.45", "0.00000001", "-42"}
	for _, val := range ivalues {
		n, err := Fixed8DecodeString(val)
		assert.Nil(t, err)
		assert.Equal(t, val, n.String())
	}

	// Fixed8DecodeString parses number with maximal precision
	val := "123456789.12345678"
	n, err := Fixed8DecodeString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(12345678912345678), n)

	// Fixed8DecodeString parses number with non-maximal precision
	val = "901.2341"
	n, err = Fixed8DecodeString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(90123410000), n)
}

func TestFixed8UnmarshalJSON(t *testing.T) {
	var testCases = []float64{
		123.45,
		-123.45,
	}

	for _, fl := range testCases {
		str := strconv.FormatFloat(fl, 'g', -1, 64)
		expected, _ := Fixed8DecodeString(str)

		// UnmarshalJSON should decode floats
		var u1 Fixed8
		s, _ := json.Marshal(fl)
		assert.Nil(t, json.Unmarshal(s, &u1))
		assert.Equal(t, expected, u1)

		// UnmarshalJSON should decode strings
		var u2 Fixed8
		s, _ = json.Marshal(str)
		assert.Nil(t, json.Unmarshal(s, &u2))
		assert.Equal(t, expected, u2)
	}
}
