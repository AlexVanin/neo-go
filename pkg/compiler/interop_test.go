package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/compiler"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestFromAddress(t *testing.T) {
	as1 := "Aej1fe4mUgou48Zzup5j8sPrE3973cJ5oz"
	addr1, err := address.StringToUint160(as1)
	require.NoError(t, err)

	as2 := "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"
	addr2, err := address.StringToUint160(as2)
	require.NoError(t, err)

	t.Run("append 2 addresses", func(t *testing.T) {
		src := `
		package foo
 		import "github.com/CityOfZion/neo-go/pkg/interop/util"
		func Main() []byte {
			addr1 := util.FromAddress("` + as1 + `")
			addr2 := util.FromAddress("` + as2 + `")
			sum := append(addr1, addr2...)
			return sum
		}
		`

		eval(t, src, append(addr1.BytesBE(), addr2.BytesBE()...))
	})

	t.Run("append 2 addresses inline", func(t *testing.T) {
		src := `
		package foo
 		import "github.com/CityOfZion/neo-go/pkg/interop/util"
		func Main() []byte {
			addr1 := util.FromAddress("` + as1 + `")
			sum := append(addr1, util.FromAddress("` + as2 + `")...)
			return sum
		}
		`

		eval(t, src, append(addr1.BytesBE(), addr2.BytesBE()...))
	})
}

func TestAppCall(t *testing.T) {
	srcInner := `
	package foo
	func Main(args []interface{}) int {
		a := args[0].(int)
		b := args[1].(int)
		return a + b
	}
	`

	inner, err := compiler.Compile(strings.NewReader(srcInner))
	require.NoError(t, err)

	ih := hash.Hash160(inner)
	getScript := func(u util.Uint160) []byte {
		if u.Equals(ih) {
			return inner
		}
		return nil
	}

	t.Run("valid script", func(t *testing.T) {
		src := getAppCallScript(fmt.Sprintf("%#v", ih.BytesBE()))
		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.NoError(t, v.Run())

		assertResult(t, v, big.NewInt(42))
	})

	t.Run("missing script", func(t *testing.T) {
		h := ih
		h[0] = ^h[0]

		src := getAppCallScript(fmt.Sprintf("%#v", h.BytesBE()))
		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.Error(t, v.Run())
	})

	t.Run("invalid script address", func(t *testing.T) {
		src := getAppCallScript("[]byte{1, 2, 3}")

		_, err := compiler.Compile(strings.NewReader(src))
		require.Error(t, err)
	})

	t.Run("convert from string constant", func(t *testing.T) {
		src := `
		package foo
		import "github.com/CityOfZion/neo-go/pkg/interop/engine"
		const scriptHash = ` + fmt.Sprintf("%#v", string(ih.BytesBE())) + `
		func Main() int {
			x := 13
			y := 29
			result := engine.AppCall([]byte(scriptHash), []interface{}{x, y})
			return result.(int)
		}
		`

		v := vmAndCompile(t, src)
		v.SetScriptGetter(getScript)

		require.NoError(t, v.Run())

		assertResult(t, v, big.NewInt(42))
	})
}

func getAppCallScript(h string) string {
	return `
	package foo
	import "github.com/CityOfZion/neo-go/pkg/interop/engine"
	func Main() int {
		x := 13
		y := 29
		result := engine.AppCall(` + h + `, []interface{}{x, y})
		return result.(int)
	}
	`
}
