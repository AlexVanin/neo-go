/*
Package crypto provides an interface to cryptographic syscalls.
*/
package crypto

// SHA256 computes SHA256 hash of b. It uses `Neo.Crypto.SHA256` syscall.
func SHA256(b []byte) []byte {
	return nil
}

// ECDsaSecp256r1Verify checks that sig is correct msg's signature for a given pub
// (serialized public key). It uses `Neo.Crypto.VerifyWithECDsaSecp256r1` syscall.
func ECDsaSecp256r1Verify(msg []byte, pub []byte, sig []byte) bool {
	return false
}
