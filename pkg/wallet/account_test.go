package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/internal/keytestcases"
	"github.com/stretchr/testify/assert"
)

func TestNewAccount(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc, err := NewAccountFromWIF(testCase.Wif)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, testCase, acc)
	}
}

func TestDecryptAccount(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc, err := DecryptAccount(testCase.EncryptedWif, testCase.Passphrase)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, testCase, acc)
	}
}

func TestNewFromWif(t *testing.T) {
	for _, testCase := range keytestcases.Arr {
		acc, err := NewAccountFromWIF(testCase.Wif)
		if testCase.Invalid {
			assert.Error(t, err)
			continue
		}

		assert.NoError(t, err)
		compareFields(t, testCase, acc)
	}
}

func compareFields(t *testing.T, tk keytestcases.Ktype, acc *Account) {
	if want, have := tk.Address, acc.Address; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.Wif, acc.wif; want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.PublicKey, hex.EncodeToString(acc.publicKey); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
	if want, have := tk.PrivateKey, acc.privateKey.String(); want != have {
		t.Fatalf("expected %s got %s", want, have)
	}
}
