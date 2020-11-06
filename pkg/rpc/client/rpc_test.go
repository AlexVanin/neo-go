package client

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcClientTestCase struct {
	name           string
	invoke         func(c *Client) (interface{}, error)
	serverResponse string
	result         func(c *Client) interface{}
	check          func(t *testing.T, c *Client, result interface{})
}

const base64B1 = "AAAAAMDaUBp2Xcx3qPKofEEGz8Ahn/lE3Ga4eyKM65V4V5akBpil+8mEGC6Q9RknLxNkVo8C9MhF4S2VOk+BbWOEDysZsQ6fcwEAAAEAAABeEr6oeuuVWIT2+c54RYzMuXwwlQH9CAEMQA0Aa27Dv5UtU2+akRIYvj7GqUqAFIhaYwES0AGLzqLF+3ZDsBxo4Lz0z7IpUi5N6X12hUlwK9/H3cJxDrcNmoIMQG3BQSSO7M/2HxlMvQYQp8avTWlrOaHyzZOzAmB7mwAavJgT/6SPDk0IsWajCE9TvepRhD2OrXMw0xcX4N6rGn8MQOTK2STAl5zmfjHb6XnOxkdPpBNcmwHJk5ZdoY9UkorwJ4Nxxltrsx/gx1KRapK3rXFS2WAzYiqF78i6iR/9kRcMQA4fvZLXYK4DFW7Rc0bKPdyAy1p34GCHG+vXpP34QtpaSlHiSjboV1K1gLZeIKTNqddO0APUh3mtESzF8V4pvzeUEwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrwMAVwQAAAAAAAAAAgAAAICWmAAAAAAAnnxEAAAAAACwBAAAAV4Svqh665VYhPb5znhFjMy5fDCVAQBZAhjd9QUMFKqKz4WdT+QCs05nPyFWgheWpIjrDBReEr6oeuuVWIT2+c54RYzMuXwwlRPADAh0cmFuc2ZlcgwUJQWey0h406h1+RxRzt7TMNRXX95BYn1bUjgB/QgBDEAIcSUsAtRql4t+IEeo+p4+YI7bA6PG+1xxUkPIb2vNlaMl4PumjQVFT+bg2ldxCYa6zccoc4n0Gfryi82EhGpGDECR4fQDr4njo94mF6/GA+OH0Y5k735yGMEZHs96586BRp6f0AQxfmIPvLcS4Yero9p0zgVl9BDg3TxU5piRylR5DEAcjOT7JjEwNRnKgDDkXfh63Yc3MorMbdb2asTiDu0aexy5M5XcikA1jypJT4wkhxjp0rrgFZRSzeYhwV0Klz+yDECIopKxLd4p+hLHxFq07WffXd++sN0WIRWzvMJncCrJqSP8zz65r8TGFFzvZMdGelWKO7KhBOhIK6wryuWNlaDIlBMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRQLQRON768AAwAAAICWmAAAAAAAPoxEAAAAAACwBAAAAV4Svqh665VYhPb5znhFjMy5fDCVAQBdAwDodkgXAAAADBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUTwAwIdHJhbnNmZXIMFLyvQdaEx9StbuDZnalwe50fDI5mQWJ9W1I4Af0IAQxAilD5fXedNlzyOeuBAtga6KqTdu+Lld59c+pAgQpm0u9eZc5j24duCxEjLAD9kxaIYXYZtemg3zbD4ubZAfxROAxA8yRbbPVWHNZSIrxgGWUsQAswFf/eip9lgGkt2wyJQCRwkobH8/kZ9zbrzMrrbo2DOL8hIqQF/YJI3LxVhikTmwxApCPPN6J4RvWmE8cEFg8bJqH7oqKESTonP3CdR/J6lBLzKgE2CoHR2gTiQVWLCqvTPaFL/34+iRWzA63GrL6YmgxAhKpKtm+dGhHNTRjPQwQLGCxHmR6zNQcF92trQTnng/yi7Zk4GtSGaILGYKYrNgTk6GTui7jfRSWHJ/v4hnAubpQTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUC0ETje+v"

const base64TxMoveNeo = "AAIAAACAlpgAAAAAAJ58RAAAAAAAsAQAAAFeEr6oeuuVWIT2+c54RYzMuXwwlQEAWQIY3fUFDBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUTwAwIdHJhbnNmZXIMFCUFnstIeNOodfkcUc7e0zDUV1/eQWJ9W1I4Af0IAQxACHElLALUapeLfiBHqPqePmCO2wOjxvtccVJDyG9rzZWjJeD7po0FRU/m4NpXcQmGus3HKHOJ9Bn68ovNhIRqRgxAkeH0A6+J46PeJhevxgPjh9GOZO9+chjBGR7PeufOgUaen9AEMX5iD7y3EuGHq6PadM4FZfQQ4N08VOaYkcpUeQxAHIzk+yYxMDUZyoAw5F34et2HNzKKzG3W9mrE4g7tGnscuTOV3IpANY8qSU+MJIcY6dK64BWUUs3mIcFdCpc/sgxAiKKSsS3eKfoSx8RatO1n313fvrDdFiEVs7zCZ3Aqyakj/M8+ua/ExhRc72THRnpVijuyoQToSCusK8rljZWgyJQTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUC0ETje+v"

const b1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"size":1641,"nextblockhash":"0x003abea54aa3c5edba7e33fb7ca96452cb65ff8cd36ce1cdfd412a6c4d3ea38a","confirmations":6,"hash":"0xd9518e322440714b0564d6f84a9a39b527b5480e4e7f7932895777a4c8fa0a9e","version":0,"previousblockhash":"0xa496577895eb8c227bb866dc44f99f21c0cf06417ca8f2a877cc5d761a50dac0","merkleroot":"0x2b0f84636d814f3a952de145c8f4028f5664132f2719f5902e1884c9fba59806","time":1596101407001,"index":1,"nextconsensus":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","witnesses":[{"invocation":"DEANAGtuw7+VLVNvmpESGL4+xqlKgBSIWmMBEtABi86ixft2Q7AcaOC89M+yKVIuTel9doVJcCvfx93CcQ63DZqCDEBtwUEkjuzP9h8ZTL0GEKfGr01pazmh8s2TswJge5sAGryYE/+kjw5NCLFmowhPU73qUYQ9jq1zMNMXF+Deqxp/DEDkytkkwJec5n4x2+l5zsZHT6QTXJsByZOWXaGPVJKK8CeDccZba7Mf4MdSkWqSt61xUtlgM2Iqhe/Iuokf/ZEXDEAOH72S12CuAxVu0XNGyj3cgMtad+Bghxvr16T9+ELaWkpR4ko26FdStYC2XiCkzanXTtAD1Id5rREsxfFeKb83","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}],"consensusdata":{"primary":0,"nonce":"0000000000000457"},"tx":[{"hash":"0x32f9bd3a2707475407c41bf5daacf9560e25ed74f6d85b3afb2ef72edb2325ba","size":555,"version":0,"nonce":2,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"10000000","netfee":"4488350","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"Ahjd9QUMFKqKz4WdT+QCs05nPyFWgheWpIjrDBReEr6oeuuVWIT2+c54RYzMuXwwlRPADAh0cmFuc2ZlcgwUJQWey0h406h1+RxRzt7TMNRXX95BYn1bUjg=","witnesses":[{"invocation":"DEAIcSUsAtRql4t+IEeo+p4+YI7bA6PG+1xxUkPIb2vNlaMl4PumjQVFT+bg2ldxCYa6zccoc4n0Gfryi82EhGpGDECR4fQDr4njo94mF6/GA+OH0Y5k735yGMEZHs96586BRp6f0AQxfmIPvLcS4Yero9p0zgVl9BDg3TxU5piRylR5DEAcjOT7JjEwNRnKgDDkXfh63Yc3MorMbdb2asTiDu0aexy5M5XcikA1jypJT4wkhxjp0rrgFZRSzeYhwV0Klz+yDECIopKxLd4p+hLHxFq07WffXd++sN0WIRWzvMJncCrJqSP8zz65r8TGFFzvZMdGelWKO7KhBOhIK6wryuWNlaDI","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]},{"hash":"0xd35d6386ec2f29b90839536f6af9466098d1665e951cdd0a20db6b4629b08369","size":559,"version":0,"nonce":3,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"10000000","netfee":"4492350","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"AwDodkgXAAAADBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUTwAwIdHJhbnNmZXIMFLyvQdaEx9StbuDZnalwe50fDI5mQWJ9W1I4","witnesses":[{"invocation":"DECKUPl9d502XPI564EC2BroqpN274uV3n1z6kCBCmbS715lzmPbh24LESMsAP2TFohhdhm16aDfNsPi5tkB/FE4DEDzJFts9VYc1lIivGAZZSxACzAV/96Kn2WAaS3bDIlAJHCShsfz+Rn3NuvMyutujYM4vyEipAX9gkjcvFWGKRObDECkI883onhG9aYTxwQWDxsmofuiooRJOic/cJ1H8nqUEvMqATYKgdHaBOJBVYsKq9M9oUv/fj6JFbMDrcasvpiaDECEqkq2b50aEc1NGM9DBAsYLEeZHrM1BwX3a2tBOeeD/KLtmTga1IZogsZgpis2BOToZO6LuN9FJYcn+/iGcC5u","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]}]}}`

const base64Header1 = "AAAAAMDaUBp2Xcx3qPKofEEGz8Ahn/lE3Ga4eyKM65V4V5akBpil+8mEGC6Q9RknLxNkVo8C9MhF4S2VOk+BbWOEDysZsQ6fcwEAAAEAAABeEr6oeuuVWIT2+c54RYzMuXwwlQH9CAEMQA0Aa27Dv5UtU2+akRIYvj7GqUqAFIhaYwES0AGLzqLF+3ZDsBxo4Lz0z7IpUi5N6X12hUlwK9/H3cJxDrcNmoIMQG3BQSSO7M/2HxlMvQYQp8avTWlrOaHyzZOzAmB7mwAavJgT/6SPDk0IsWajCE9TvepRhD2OrXMw0xcX4N6rGn8MQOTK2STAl5zmfjHb6XnOxkdPpBNcmwHJk5ZdoY9UkorwJ4Nxxltrsx/gx1KRapK3rXFS2WAzYiqF78i6iR/9kRcMQA4fvZLXYK4DFW7Rc0bKPdyAy1p34GCHG+vXpP34QtpaSlHiSjboV1K1gLZeIKTNqddO0APUh3mtESzF8V4pvzeUEwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrwA="

const header1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"hash":"0xd9518e322440714b0564d6f84a9a39b527b5480e4e7f7932895777a4c8fa0a9e","size":518,"version":0,"previousblockhash":"0xa496577895eb8c227bb866dc44f99f21c0cf06417ca8f2a877cc5d761a50dac0","merkleroot":"0x2b0f84636d814f3a952de145c8f4028f5664132f2719f5902e1884c9fba59806","time":1596101407001,"index":1,"nextconsensus":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","witnesses":[{"invocation":"DEANAGtuw7+VLVNvmpESGL4+xqlKgBSIWmMBEtABi86ixft2Q7AcaOC89M+yKVIuTel9doVJcCvfx93CcQ63DZqCDEBtwUEkjuzP9h8ZTL0GEKfGr01pazmh8s2TswJge5sAGryYE/+kjw5NCLFmowhPU73qUYQ9jq1zMNMXF+Deqxp/DEDkytkkwJec5n4x2+l5zsZHT6QTXJsByZOWXaGPVJKK8CeDccZba7Mf4MdSkWqSt61xUtlgM2Iqhe/Iuokf/ZEXDEAOH72S12CuAxVu0XNGyj3cgMtad+Bghxvr16T9+ELaWkpR4ko26FdStYC2XiCkzanXTtAD1Id5rREsxfFeKb83","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}],"confirmations":6,"nextblockhash":"0x003abea54aa3c5edba7e33fb7ca96452cb65ff8cd36ce1cdfd412a6c4d3ea38a"}}`

const txMoveNeoVerbose = `{"id":5,"jsonrpc":"2.0","result":{"blockhash":"0xd9518e322440714b0564d6f84a9a39b527b5480e4e7f7932895777a4c8fa0a9e","confirmations":6,"blocktime":1596101407001,"vmstate":"","hash":"0x32f9bd3a2707475407c41bf5daacf9560e25ed74f6d85b3afb2ef72edb2325ba","size":555,"version":0,"nonce":2,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"10000000","netfee":"4488350","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"Ahjd9QUMFKqKz4WdT+QCs05nPyFWgheWpIjrDBReEr6oeuuVWIT2+c54RYzMuXwwlRPADAh0cmFuc2ZlcgwUJQWey0h406h1+RxRzt7TMNRXX95BYn1bUjg=","witnesses":[{"invocation":"DEAIcSUsAtRql4t+IEeo+p4+YI7bA6PG+1xxUkPIb2vNlaMl4PumjQVFT+bg2ldxCYa6zccoc4n0Gfryi82EhGpGDECR4fQDr4njo94mF6/GA+OH0Y5k735yGMEZHs96586BRp6f0AQxfmIPvLcS4Yero9p0zgVl9BDg3TxU5piRylR5DEAcjOT7JjEwNRnKgDDkXfh63Yc3MorMbdb2asTiDu0aexy5M5XcikA1jypJT4wkhxjp0rrgFZRSzeYhwV0Klz+yDECIopKxLd4p+hLHxFq07WffXd++sN0WIRWzvMJncCrJqSP8zz65r8TGFFzvZMdGelWKO7KhBOhIK6wryuWNlaDI","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]}}`

// getResultBlock1 returns data for block number 1 which is used by several tests.
func getResultBlock1() *result.Block {
	binB, err := base64.StdEncoding.DecodeString(base64B1)
	if err != nil {
		panic(err)
	}
	b := block.New(netmode.UnitTestNet)
	err = testserdes.DecodeBinary(binB, b)
	if err != nil {
		panic(err)
	}
	b2Hash, err := util.Uint256DecodeStringLE("003abea54aa3c5edba7e33fb7ca96452cb65ff8cd36ce1cdfd412a6c4d3ea38a")
	if err != nil {
		panic(err)
	}
	return &result.Block{
		Block: *b,
		BlockMetadata: result.BlockMetadata{
			Size:          1641,
			NextBlockHash: &b2Hash,
			Confirmations: 6,
		},
	}
}

func getTxMoveNeo() *result.TransactionOutputRaw {
	b1 := getResultBlock1()
	txBin, err := base64.StdEncoding.DecodeString(base64TxMoveNeo)
	if err != nil {
		panic(err)
	}
	tx, err := transaction.NewTransactionFromBytes(netmode.UnitTestNet, txBin)
	if err != nil {
		panic(err)
	}
	return &result.TransactionOutputRaw{
		Transaction: *tx,
		TransactionMetadata: result.TransactionMetadata{
			Timestamp:     b1.Timestamp,
			Blockhash:     b1.Block.Hash(),
			Confirmations: int(b1.Confirmations),
		},
	}
}

// rpcClientTestCases contains `serverResponse` json data fetched from examples
// published in official C# JSON-RPC API v2.10.3 reference
// (see https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
var rpcClientTestCases = map[string][]rpcClientTestCase{
	"getapplicationlog": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}}`,
			result: func(c *Client) interface{} {
				txHash, err := util.Uint256DecodeStringLE("17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521")
				if err != nil {
					panic(err)
				}
				return &state.AppExecResult{
					TxHash:      txHash,
					Trigger:     trigger.Application,
					VMState:     vm.HaltState,
					GasConsumed: 1,
					Stack:       []stackitem.Item{stackitem.NewBigInteger(big.NewInt(1))},
					Events:      []state.NotificationEvent{},
				}
			},
		},
	},
	"getbestblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e"}`,
			result: func(c *Client) interface{} {
				result, err := util.Uint256DecodeStringLE("773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e")
				if err != nil {
					panic(err)
				}
				return result
			},
		},
	},
	"getblock": {
		{
			name: "byIndex_positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64B1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &b.Block
			},
		},
		{
			name: "byIndex_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetBlockByIndexVerbose(1)
			},
			serverResponse: b1Verbose,
			result: func(c *Client) interface{} {
				res := getResultBlock1()
				// update hidden hash value.
				_ = res.Block.ConsensusData.Hash()
				return res
			},
		},
		{
			name: "byHash_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("d151651e86680a7ecbc87babf3346a42e7bc9974414ce192c9c22ac4f2e9d043")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64B1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &b.Block
			},
		},
		{
			name: "byHash_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHashVerbose(hash)
			},
			serverResponse: b1Verbose,
			result: func(c *Client) interface{} {
				res := getResultBlock1()
				// update hidden hash value.
				_ = res.Block.ConsensusData.Hash()
				return res
			},
		},
	},
	"getblockcount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":991991}`,
			result: func(c *Client) interface{} {
				return uint32(991991)
			},
		},
	},
	"getblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2"}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2")
				if err != nil {
					panic(err)
				}
				return hash
			},
		},
	},
	"getblockheader": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("68e4bd688b852e807eef13a0ff7da7b02223e359a35153667e88f9cb4a3b0801")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64Header1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return b.Header()
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeaderVerbose(hash)
			},
			serverResponse: header1Verbose,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &result.Header{
					Hash:          b.Hash(),
					Size:          518,
					Version:       b.Version,
					NextBlockHash: b.NextBlockHash,
					PrevBlockHash: b.PrevHash,
					MerkleRoot:    b.MerkleRoot,
					Timestamp:     b.Timestamp,
					Index:         b.Index,
					NextConsensus: address.Uint160ToString(b.NextConsensus),
					Witnesses:     []transaction.Witness{b.Script},
					Confirmations: 6,
				}
			},
		},
	},
	"getblocksysfee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"195500"}`,
			result: func(c *Client) interface{} {
				return util.Fixed8FromInt64(195500)
			},
		},
	},
	"getcommittee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCommittee()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e"]}`,
			result: func(c *Client) interface{} {
				member, err := keys.NewPublicKeyFromString("02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e")
				if err != nil {
					panic(fmt.Errorf("failed to decode public key: %w", err))
				}
				return keys.PublicKeys{member}
			},
		},
	},
	"getconnectioncount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":10}`,
			result: func(c *Client) interface{} {
				return 10
			},
		},
	},
	"getcontractstate": {
		{
			name: "positive, by hash",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1b4357bff5a01bdf2a6581247cf9ed1e24629176")
				if err != nil {
					panic(err)
				}
				return c.GetContractStateByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","manifest":{"abi":{"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","methods":[],"events":[]},"groups":[],"features":{"payable":false,"storage":true},"permissions":null,"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest(hash.Hash160(script))
				m.Features = smartcontract.HasStorage
				cs := &state.Contract{
					ID:       0,
					Script:   script,
					Manifest: *m,
				}
				_ = cs.ScriptHash()
				return cs
			},
		},
		{
			name: "positive, by address",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByAddressOrName("NWiu5oejTu925aeL9Hc1LX8SvaJhE23h15")
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","manifest":{"abi":{"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","methods":[],"events":[]},"groups":[],"features":{"payable":false,"storage":true},"permissions":null,"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest(hash.Hash160(script))
				m.Features = smartcontract.HasStorage
				cs := &state.Contract{
					ID:       0,
					Script:   script,
					Manifest: *m,
				}
				_ = cs.ScriptHash()
				return cs
			},
		},
		{
			name: "positive, by id",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByID(0)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","manifest":{"abi":{"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","methods":[],"events":[]},"groups":[],"features":{"payable":false,"storage":true},"permissions":null,"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest(hash.Hash160(script))
				m.Features = smartcontract.HasStorage
				cs := &state.Contract{
					ID:       0,
					Script:   script,
					Manifest: *m,
				}
				_ = cs.ScriptHash()
				return cs
			},
		},
	},
	"getFeePerByte": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetFeePerByte()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"1000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(1000)
			},
		},
	},
	"getMaxTransacctionsPerBlock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetMaxTransactionsPerBlock()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMGmdldE1heFRyYW5zYWN0aW9uc1BlckJsb2NrDBSaYaRu7Je4kwbXzoHxW0YgkdAJMkFifVtS","stack":[{"type":"Integer","value":"512"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(512)
			},
		},
	},
	"getMaxBlockSize": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetMaxBlockSize()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMD2dldE1heEJsb2NrU2l6ZQwUmmGkbuyXuJMG186B8VtGIJHQCTJBYn1bUg==","stack":[{"type":"Integer","value":"262144"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(262144)
			},
		},
	},
	"isBlocked": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.IsBlocked(util.Uint160{1, 2, 3})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMEmdldEJsb2NrZWRBY2NvdW50cwwUmmGkbuyXuJMG186B8VtGIJHQCTJBYn1bUg==","stack":[{"type":"Boolean","value":false}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return false
			},
		},
	},
	"getnep5balances": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP5Balances(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"assethash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","amount":"50000000000","lastupdatedblock":251604}],"address":"AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Balances{
					Balances: []result.NEP5Balance{{
						Asset:       hash,
						Amount:      "50000000000",
						LastUpdated: 251604,
					}},
					Address: "AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w",
				}
			},
		},
	},
	"getnep5transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF", nil, nil, nil, nil)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"assethash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transferaddress":"AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis","amount":"1000000","blockindex":436036,"transfernotifyindex":0,"txhash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF"}}`,
			result: func(c *Client) interface{} {
				assetHash, err := util.Uint160DecodeStringLE("600c4f5200db36177e3e8a09e9f18e2fc7d12a0f")
				if err != nil {
					panic(err)
				}
				txHash, err := util.Uint256DecodeStringLE("df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Transfers{
					Sent: []result.NEP5Transfer{},
					Received: []result.NEP5Transfer{
						{
							Timestamp:   1555651816,
							Asset:       assetHash,
							Address:     "AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis",
							Amount:      "1000000",
							Index:       436036,
							NotifyIndex: 0,
							TxHash:      txHash,
						},
					},
					Address: "AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF",
				}
			},
		},
	},
	"getpeers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"unconnected":[{"address":"172.200.0.1","port":"20333"}],"connected":[{"address":"127.0.0.1","port":"20335"}],"bad":[{"address":"172.200.0.254","port":"20332"}]}}`,
			result: func(c *Client) interface{} {
				return &result.GetPeers{
					Unconnected: result.Peers{
						{
							Address: "172.200.0.1",
							Port:    "20333",
						},
					},
					Connected: result.Peers{
						{
							Address: "127.0.0.1",
							Port:    "20335",
						},
					},
					Bad: result.Peers{
						{
							Address: "172.200.0.254",
							Port:    "20332",
						},
					},
				}
			},
		},
	},
	"getrawmempool": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["0x9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e"]}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e")
				if err != nil {
					panic(err)
				}
				return []util.Uint256{hash}
			},
		},
	},
	"getrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("ca23bd5df3249836849309ca2afe972bfd288b0a7ae61302c8fd545daa8bffd6")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64TxMoveNeo + `"}`,
			result: func(c *Client) interface{} {
				tx := getTxMoveNeo()
				return &tx.Transaction
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("8185b0db7ed77190b93ac8bd44896822cd8f3cfcf702b3f50131e0efd200ef96")
				if err != nil {
					panic(err)
				}
				out, err := c.GetRawTransactionVerbose(hash)
				if err != nil {
					return nil, err
				}
				out.Transaction.FeePerByte() // set fee per byte
				return out, nil
			},
			serverResponse: txMoveNeoVerbose,
			result: func(c *Client) interface{} {
				return getTxMoveNeo()
			},
		},
	},
	"getstorage": {
		{
			name: "by hash, positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByHash(hash, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"4c696e"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
		{
			name: "by ID, positive",
			invoke: func(c *Client) (interface{}, error) {
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByID(-1, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"4c696e"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
	},
	"gettransactionheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetTransactionHeight(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":1}`,
			result: func(c *Client) interface{} {
				return uint32(1)
			},
		},
	},
	"getunclaimedgas": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB","unclaimed":"897299680935"}}`,
			result: func(c *Client) interface{} {
				addr, err := address.StringToUint160("NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB")
				if err != nil {
					panic(fmt.Errorf("failed to parse UnclaimedGas address: %w", err))
				}
				return result.UnclaimedGas{
					Address:   addr,
					Unclaimed: *big.NewInt(897299680935),
				}
			},
		},
	},
	"getvalidators": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNextBlockValidators()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`,
			result:         func(c *Client) interface{} { return []result.Validator{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.([]result.Validator)
				require.True(t, ok)
				assert.Equal(t, 4, len(res))
			},
		},
	},
	"getversion": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"magic":42,"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
			result: func(c *Client) interface{} {
				return &result.Version{
					Magic:     netmode.UnitTestNet,
					TCPPort:   uint16(20332),
					WSPort:    uint16(20342),
					Nonce:     2153672787,
					UserAgent: "/NEO-GO:0.73.1-pre-273-ge381358/",
				}
			},
		},
	},
	"invokefunction": {
		{
			name: "positive, by scripthash",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				contr, err := util.Uint160DecodeStringLE("af7c7328eee5a275a3bcaee2bf0cf662b5e739be")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction(contr, "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				}, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv","state":"HALT","gasconsumed":"31100000","stack":[{"type":"ByteString","value":"JivsCEQy"}],"tx":"000800000080969800000000000204130000000000b004000001aa8acf859d4fe402b34e673f2156821796a488eb01005701e8030c14aa8acf859d4fe402b34e673f2156821796a488eb0c14e79eb66d3c134a4a776ee807d2e5b846dda4fdb013c00c087472616e736665720c14e79eb66d3c134a4a776ee807d2e5b846dda4fdb041627d5b523801420c40d83408774d4bd8b19ae870561d06f2744eb7678f58d1b90cf2f50e98ae83f60b0824e2feeadef6de6418a4cfc43bbc1f916c33ec594cbe662a9d924786e17a14290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b4195440d78"}}`,
			result: func(c *Client) interface{} {
				return &result.Invoke{}
			},
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Invoke)
				require.True(t, ok)
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv")
				if err != nil {
					panic(err)
				}
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, int64(31100000), res.GasConsumed)
				assert.Equal(t, script, res.Script)
				assert.Equal(t, []stackitem.Item{stackitem.NewByteArray(bytes)}, res.Stack)
				assert.NotNil(t, res.Transaction)
			},
		},
		{
			name: "positive, FAULT state",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				contr, err := util.Uint160DecodeStringLE("af7c7328eee5a275a3bcaee2bf0cf662b5e739be")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction(contr, "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				}, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv","state":"FAULT","gasconsumed":"31100000","stack":[{"type":"ByteString","value":"JivsCEQy"}],"tx":"000800000080969800000000000204130000000000b004000001aa8acf859d4fe402b34e673f2156821796a488eb01005701e8030c14aa8acf859d4fe402b34e673f2156821796a488eb0c14e79eb66d3c134a4a776ee807d2e5b846dda4fdb013c00c087472616e736665720c14e79eb66d3c134a4a776ee807d2e5b846dda4fdb041627d5b523801420c40d83408774d4bd8b19ae870561d06f2744eb7678f58d1b90cf2f50e98ae83f60b0824e2feeadef6de6418a4cfc43bbc1f916c33ec594cbe662a9d924786e17a14290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b4195440d78","exception":"gas limit exceeded"}}`,
			result: func(c *Client) interface{} {
				return &result.Invoke{}
			},
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Invoke)
				require.True(t, ok)
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv")
				if err != nil {
					panic(err)
				}
				assert.Equal(t, "FAULT", res.State)
				assert.Equal(t, int64(31100000), res.GasConsumed)
				assert.Equal(t, script, res.Script)
				assert.Equal(t, []stackitem.Item{stackitem.NewByteArray(bytes)}, res.Stack)
				assert.NotNil(t, res.Transaction)
			},
		},
	},
	"invokescript": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				script, err := base64.StdEncoding.DecodeString("AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR")
				if err != nil {
					panic(err)
				}
				return c.InvokeScript(script, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR","state":"HALT","gasconsumed":"16100000","stack":[{"type":"ByteString","value":"TkVQNSBHQVM="}],"tx":null}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("4e45503520474153")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: 16100000,
					Script:      script,
					Stack:       []stackitem.Item{stackitem.NewByteArray(bytes)},
				}
			},
		},
	},
	"sendrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"hash":"0x72159b0cf1221110daad6e1df6ef4ff03012173b63c86910bd7134deb659c875"}}`,
			result: func(c *Client) interface{} {
				h, err := util.Uint256DecodeStringLE("72159b0cf1221110daad6e1df6ef4ff03012173b63c86910bd7134deb659c875")
				if err != nil {
					panic(fmt.Errorf("can't decode `sendrawtransaction` result hash: %w", err))
				}
				return h
			},
		},
	},
	"submitblock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"hash":"0x1bdea8f80eb5bd97fade38d5e7fb93b02c9d3e01394e9f4324218132293f7ea6"}}`,
			result: func(c *Client) interface{} {
				h, err := util.Uint256DecodeStringLE("1bdea8f80eb5bd97fade38d5e7fb93b02c9d3e01394e9f4324218132293f7ea6")
				if err != nil {
					panic(fmt.Errorf("can't decode `submitblock` result hash: %w", err))
				}
				return h
			},
		},
	},
	"validateaddress": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i","isvalid":true}}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
}

type rpcClientErrorCase struct {
	name   string
	invoke func(c *Client) (interface{}, error)
}

var rpcClientErrorCases = map[string][]rpcClientErrorCase{
	`{"jsonrpc":"2.0","id":1,"result":"not-a-hex-string"}`: {
		{
			name: "getblock_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblockheader_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
		{
			name: "getstoragebyhash_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByHash(hash, key)
			},
		},
		{
			name: "getstoragebyid_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByID(-1, key)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":"01"}`: {
		{
			name: "getblock_decodebin_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getheader_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":false}`: {
		{
			name: "sendrawtransaction_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
		},
	},
	`{"id":1,"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"}}`: {
		{
			name: "getapplicationlog_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(0)
			},
		},
		{
			name: "getblockheader_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByHash(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("", nil, nil, nil, nil)
			},
		},
		{
			name: "getnep5transfers_invalid_params_error 2",
			invoke: func(c *Client) (interface{}, error) {
				var stop uint32
				return c.GetNEP5Transfers("NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", nil, &stop, nil, nil)
			},
		},
		{
			name: "getnep5transfers_invalid_params_error 3",
			invoke: func(c *Client) (interface{}, error) {
				var start uint32
				var limit int
				return c.GetNEP5Transfers("NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", &start, nil, &limit, nil)
			},
		},
		{
			name: "getnep5transfers_invalid_params_error 4",
			invoke: func(c *Client) (interface{}, error) {
				var start, stop uint32
				var page int
				return c.GetNEP5Transfers("NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc", &start, &stop, nil, &page)
			},
		},
		{
			name: "getrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstoragebyhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByHash(util.Uint160{}, []byte{})
			},
		},
		{
			name: "getstoragebyid_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByID(-1, []byte{})
			},
		},
		{
			name: "gettransactionheight_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "invokefunction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction(util.Uint160{}, "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript([]byte{}, nil)
			},
		},
		{
			name: "sendrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(&transaction.Transaction{})
			},
		},
		{
			name: "submitblock_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{})
			},
		},
		{
			name: "validateaddress_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
	`{}`: {
		{
			name: "getapplicationlog_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockcount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
		},
		{
			name: "getblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
		},
		{
			name: "getblockheader_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getcommittee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCommittee()
			},
		},
		{
			name: "getconnectioncount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByHash(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("", nil, nil, nil, nil)
			},
		},
		{
			name: "getpeers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
		},
		{
			name: "getrawmempool_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
		},
		{
			name: "getrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstoragebyhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByHash(util.Uint160{}, []byte{})
			},
		},
		{
			name: "getstoragebyid_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByID(-1, []byte{})
			},
		},
		{
			name: "gettransactionheight_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "getvalidators_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNextBlockValidators()
			},
		},
		{
			name: "invokefunction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction(util.Uint160{}, "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript([]byte{}, nil)
			},
		},
		{
			name: "sendrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
}

func TestRPCClients(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			c, err := New(ctx, endpoint, opts)
			require.NoError(t, err)
			require.NoError(t, c.Init())
			return c, nil
		})
	})
	t.Run("WSClient", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			wsc, err := NewWS(ctx, httpURLtoWS(endpoint), opts)
			require.NoError(t, err)
			require.NoError(t, wsc.Init())
			return &wsc.Client, nil
		})
	})
}

func testRPCClient(t *testing.T, newClient func(context.Context, string, Options) (*Client, error)) {
	for method, testBatch := range rpcClientTestCases {
		t.Run(method, func(t *testing.T) {
			for _, testCase := range testBatch {
				t.Run(testCase.name, func(t *testing.T) {
					srv := initTestServer(t, testCase.serverResponse)
					defer srv.Close()

					endpoint := srv.URL
					opts := Options{}
					c, err := newClient(context.TODO(), endpoint, opts)
					if err != nil {
						t.Fatal(err)
					}

					actual, err := testCase.invoke(c)
					assert.NoError(t, err)

					expected := testCase.result(c)
					if testCase.check == nil {
						assert.Equal(t, expected, actual)
					} else {
						testCase.check(t, c, actual)
					}
				})
			}
		})
	}
	for serverResponse, testBatch := range rpcClientErrorCases {
		srv := initTestServer(t, serverResponse)
		defer srv.Close()

		endpoint := srv.URL
		opts := Options{}
		c, err := newClient(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, testCase := range testBatch {
			t.Run(testCase.name, func(t *testing.T) {
				_, err := testCase.invoke(c)
				assert.Error(t, err)
			})
		}
	}
}

func httpURLtoWS(url string) string {
	return "ws" + strings.TrimPrefix(url, "http") + "/ws"
}

func initTestServer(t *testing.T, resp string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for {
				ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, p, err := ws.ReadMessage()
				if err != nil {
					break
				}
				r := request.NewIn()
				err = json.Unmarshal(p, r)
				if err != nil {
					t.Fatalf("Cannot decode request body: %s", req.Body)
				}
				response := wrapInitResponse(r, resp)
				ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err = ws.WriteMessage(1, []byte(response))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
		r := request.NewIn()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		requestHandler(t, r, w, resp)
	}))

	return srv
}

func requestHandler(t *testing.T, r *request.In, w http.ResponseWriter, resp string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := wrapInitResponse(r, resp)
	_, err := w.Write([]byte(response))
	if err != nil {
		t.Fatalf("Error writing response: %s", err.Error())
	}
}

func wrapInitResponse(r *request.In, resp string) string {
	var response string
	switch r.Method {
	case "getversion":
		response = `{"id":1,"jsonrpc":"2.0","result":{"magic":42,"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`
	case "getcontractstate":
		p, err := r.Params()
		if err != nil {
			response = resp
		}
		name, err := p.ValueWithType(0, request.StringT).GetString()
		if err != nil {
			response = resp
		}
		switch name {
		case "neo":
			response = `{"id":1,"jsonrpc":"2.0","result":{"id":-1,"script":"DANORU9Ba2d4Cw==","manifest":{"abi":{"hash":"0xde5f57d430d3dece511cf975a8d37848cb9e0525","methods":[{"name":"name","offset":0,"parameters":null,"returntype":"String"},{"name":"symbol","offset":0,"parameters":null,"returntype":"String"},{"name":"decimals","offset":0,"parameters":null,"returntype":"Integer"},{"name":"totalSupply","offset":0,"parameters":null,"returntype":"Integer"},{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer"},{"name":"transfer","offset":0,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}],"returntype":"Boolean"},{"name":"onPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"postPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"unclaimedGas","offset":0,"parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer"},{"name":"registerCandidate","offset":0,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"unregisterCandidate","offset":0,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"vote","offset":0,"parameters":[{"name":"account","type":"Hash160"},{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"getCandidates","offset":0,"parameters":null,"returntype":"Array"},{"name":"getСommittee","offset":0,"parameters":null,"returntype":"Array"},{"name":"getNextBlockValidators","offset":0,"parameters":null,"returntype":"Array"},{"name":"getGasPerBlock","offset":0,"parameters":null,"returntype":"Integer"},{"name":"setGasPerBlock","offset":0,"parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Boolean"}],"events":[{"name":"Transfer","parameters":null}]},"groups":[],"features":{"payable":false,"storage":false},"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-5"],"trusts":[],"safemethods":["name","symbol","decimals","totalSupply","balanceOf","unclaimedGas","getCandidates","getСommittee","getNextBlockValidators"],"extra":null},"hash":"0xde5f57d430d3dece511cf975a8d37848cb9e0525"}}`
		case "gas":
			response = `{"id":1,"jsonrpc":"2.0","result":{"id":-2,"script":"DANHQVNBa2d4Cw==","manifest":{"abi":{"hash":"0x668e0c1f9d7b70a99dd9e06eadd4c784d641afbc","methods":[{"name":"name","offset":0,"parameters":null,"returntype":"String"},{"name":"symbol","offset":0,"parameters":null,"returntype":"String"},{"name":"decimals","offset":0,"parameters":null,"returntype":"Integer"},{"name":"totalSupply","offset":0,"parameters":null,"returntype":"Integer"},{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer"},{"name":"transfer","offset":0,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}],"returntype":"Boolean"},{"name":"onPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"postPersist","offset":0,"parameters":null,"returntype":"Void"}],"events":[{"name":"Transfer","parameters":null}]},"groups":[],"features":{"payable":false,"storage":false},"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-5"],"trusts":[],"safemethods":["name","symbol","decimals","totalSupply","balanceOf"],"extra":null},"hash":"0x668e0c1f9d7b70a99dd9e06eadd4c784d641afbc"}}`
		default:
			response = resp
		}
	default:
		response = resp
	}
	return response
}

func TestCalculateValidUntilBlock(t *testing.T) {
	var (
		getBlockCountCalled int
		getValidatorsCalled int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := request.NewIn()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		var response string
		switch r.Method {
		case "getblockcount":
			getBlockCountCalled++
			response = `{"jsonrpc":"2.0","id":1,"result":50}`
		case "getnextblockvalidators":
			getValidatorsCalled++
			response = `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`
		}
		requestHandler(t, r, w, response)
	}))
	defer srv.Close()

	endpoint := srv.URL
	opts := Options{}
	c, err := New(context.TODO(), endpoint, opts)
	if err != nil {
		t.Fatal(err)
	}
	require.NoError(t, c.Init())

	validUntilBlock, err := c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(55), validUntilBlock)
	assert.Equal(t, 1, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)

	// check, whether caching is working
	validUntilBlock, err = c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(55), validUntilBlock)
	assert.Equal(t, 2, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)
}

func TestGetNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := request.NewIn()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		// request handler already have `getversion` response wrapper
		requestHandler(t, r, w, "")
	}))
	defer srv.Close()
	endpoint := srv.URL
	opts := Options{}

	t.Run("bad", func(t *testing.T) {
		c, err := New(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}
		// network was not initialised
		require.Equal(t, netmode.Magic(0), c.GetNetwork())
		require.Equal(t, false, c.initDone)
	})

	t.Run("good", func(t *testing.T) {
		c, err := New(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}
		require.NoError(t, c.Init())
		require.Equal(t, netmode.UnitTestNet, c.GetNetwork())
	})
}
