/*
Package std provides interface to StdLib native contract.
It implements various useful conversion functions.
*/
package std

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents StdLib contract hash.
const Hash = "\xc0\xef\x39\xce\xe0\xe4\xe9\x25\xc6\xc2\xa0\x6a\x79\xe1\x44\x0d\xd8\x6f\xce\xac"

// Serialize calls `serialize` method of StdLib native contract and serializes
// any given item into a byte slice. It works for all regular VM types (not ones
// from interop package) and allows to save them in storage or pass into Notify
// and then Deserialize them on the next run or in the external event receiver.
func Serialize(item interface{}) []byte {
	return contract.Call(interop.Hash160(Hash), "serialize", contract.NoneFlag,
		item).([]byte)
}

// Deserialize calls `deserialize` method of StdLib native contract and unpacks
// previously serialized value from a byte slice, it's the opposite of Serialize.
func Deserialize(b []byte) interface{} {
	return contract.Call(interop.Hash160(Hash), "deserialize", contract.NoneFlag,
		b)
}

// JSONSerialize serializes value to json. It uses `jsonSerialize` method of StdLib native
// contract.
// Serialization format is the following:
// []byte -> base64 string
// bool -> json boolean
// nil -> Null
// string -> base64 encoded sequence of underlying bytes
// (u)int* -> integer, only value in -2^53..2^53 are allowed
// []interface{} -> json array
// map[type1]type2 -> json object with string keys marshaled as strings (not base64).
func JSONSerialize(item interface{}) []byte {
	return contract.Call(interop.Hash160(Hash), "jsonSerialize", contract.NoneFlag,
		item).([]byte)
}

// JSONDeserialize deserializes value from json. It uses `jsonDeserialize` method of StdLib
// native contract.
// It performs deserialization as follows:
// strings -> []byte (string) from base64
// integers -> (u)int* types
// null -> interface{}(nil)
// arrays -> []interface{}
// maps -> map[string]interface{}
func JSONDeserialize(data []byte) interface{} {
	return contract.Call(interop.Hash160(Hash), "jsonDeserialize", contract.NoneFlag,
		data)
}

// Base64Encode calls `base64Encode` method of StdLib native contract and encodes
// given byte slice into a base64 string and returns byte representation of this
// string.
func Base64Encode(b []byte) string {
	return contract.Call(interop.Hash160(Hash), "base64Encode", contract.NoneFlag,
		b).(string)
}

// Base64Decode calls `base64Decode` method of StdLib native contract and decodes
// given base64 string represented as a byte slice into byte slice.
func Base64Decode(b []byte) []byte {
	return contract.Call(interop.Hash160(Hash), "base64Decode", contract.NoneFlag,
		b).([]byte)
}

// Base58Encode calls `base58Encode` method of StdLib native contract and encodes
// given byte slice into a base58 string and returns byte representation of this
// string.
func Base58Encode(b []byte) string {
	return contract.Call(interop.Hash160(Hash), "base58Encode", contract.NoneFlag,
		b).(string)
}

// Base58Decode calls `base58Decode` method of StdLib native contract and decodes
// given base58 string represented as a byte slice into a new byte slice.
func Base58Decode(b []byte) []byte {
	return contract.Call(interop.Hash160(Hash), "base58Decode", contract.NoneFlag,
		b).([]byte)
}

// Itoa converts num in a given base to string. Base should be either 10 or 16.
// It uses `itoa` method of StdLib native contract.
func Itoa(num int, base int) string {
	return contract.Call(interop.Hash160(Hash), "itoa", contract.NoneFlag,
		num, base).(string)
}

// Atoi converts string to a number in a given base. Base should be either 10 or 16.
// It uses `atoi` method of StdLib native contract.
func Atoi(s string, base int) int {
	return contract.Call(interop.Hash160(Hash), "atoi", contract.NoneFlag,
		s, base).(int)
}
