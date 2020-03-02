package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/emit"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var privNetKeys = []string{
	"KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY",
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",
}

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t *testing.T) *Blockchain {
	unitTestNetCfg, err := config.Load("../../config", config.ModeUnitTestNet)
	require.NoError(t, err)
	chain, err := NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err)
	go chain.Run()
	return chain
}

func (bc *Blockchain) newBlock(txs ...*transaction.Transaction) *block.Block {
	lastBlock := bc.topBlock.Load().(*block.Block)
	return newBlock(bc.config, lastBlock.Index+1, lastBlock.Hash(), txs...)
}

func newBlock(cfg config.ProtocolConfiguration, index uint32, prev util.Uint256, txs ...*transaction.Transaction) *block.Block {
	validators, _ := getValidators(cfg)
	vlen := len(validators)
	valScript, _ := smartcontract.CreateMultiSigRedeemScript(
		vlen-(vlen-1)/3,
		validators,
	)
	witness := transaction.Witness{
		VerificationScript: valScript,
	}
	b := &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      prev,
			Timestamp:     uint32(time.Now().UTC().Unix()) + index,
			Index:         index,
			ConsensusData: 1111,
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		Transactions: txs,
	}
	_ = b.RebuildMerkleRoot()

	invScript := make([]byte, 0)
	for _, wif := range privNetKeys {
		pKey, err := keys.NewPrivateKeyFromWIF(wif)
		if err != nil {
			panic(err)
		}
		b := b.GetHashableData()
		sig := pKey.Sign(b)
		if len(sig) != 64 {
			panic("wrong signature length")
		}
		invScript = append(invScript, byte(opcode.PUSHBYTES64))
		invScript = append(invScript, sig...)
	}
	b.Script.InvocationScript = invScript
	return b
}

func (bc *Blockchain) genBlocks(n int) ([]*block.Block, error) {
	blocks := make([]*block.Block, n)
	lastHash := bc.topBlock.Load().(*block.Block).Hash()
	for i := 0; i < n; i++ {
		blocks[i] = newBlock(bc.config, uint32(i)+1, lastHash, newMinerTX())
		if err := bc.AddBlock(blocks[i]); err != nil {
			return blocks, err
		}
		lastHash = blocks[i].Hash()
	}
	return blocks, nil
}

func newMinerTX() *transaction.Transaction {
	return &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{},
	}
}

func getDecodedBlock(t *testing.T, i int) *block.Block {
	data, err := getBlockData(i)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := &block.Block{}
	r := io.NewBinReaderFromBuf(b)
	block.DecodeBinary(r)
	require.NoError(t, r.Err)

	return block
}

func getBlockData(i int) (map[string]interface{}, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("test_data/block_%d.json", i))
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, err
}

func newDumbBlock() *block.Block {
	return &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     uint32(100500),
			Index:         1,
			ConsensusData: 1111,
			NextConsensus: hash.Hash160([]byte("a")),
			Script: transaction.Witness{
				VerificationScript: []byte{0x51}, // PUSH1
				InvocationScript:   []byte{0x61}, // NOP
			},
		},
		Transactions: []*transaction.Transaction{
			{Type: transaction.MinerType},
			{Type: transaction.IssueType},
		},
	}
}

// This function generates "../rpc/testdata/testblocks.acc" file which contains data
// for RPC unit tests.
// To generate new "../rpc/testdata/testblocks.acc", follow the steps:
// 		1. Rename the function
// 		2. Add specific test-case into "neo-go/pkg/core/blockchain_test.go"
// 		3. Run tests with `$ make test`
func _(t *testing.T) {
	bc := newTestChain(t)
	n := 50
	_, err := bc.genBlocks(n)
	require.NoError(t, err)

	tx1 := newMinerTX()

	avm, err := ioutil.ReadFile("../rpc/testdata/test_contract.avm")
	require.NoError(t, err)

	var props smartcontract.PropertyState
	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("Da contract dat hallos u"))
	emit.Bytes(script.BinWriter, []byte("joe@example.com"))
	emit.Bytes(script.BinWriter, []byte("Random Guy"))
	emit.Bytes(script.BinWriter, []byte("0.99"))
	emit.Bytes(script.BinWriter, []byte("Helloer"))
	props |= smartcontract.HasStorage
	emit.Int(script.BinWriter, int64(props))
	emit.Int(script.BinWriter, int64(5))
	params := make([]byte, 1)
	params[0] = byte(7)
	emit.Bytes(script.BinWriter, params)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, "Neo.Contract.Create")
	txScript := script.Bytes()

	tx2 := transaction.NewInvocationTX(txScript, util.Fixed8FromFloat(100))

	block := bc.newBlock(tx1, tx2)
	require.NoError(t, bc.AddBlock(block))

	script = io.NewBufBinWriter()
	emit.String(script.BinWriter, "testvalue")
	emit.String(script.BinWriter, "testkey")
	emit.Int(script.BinWriter, 2)
	emit.Opcode(script.BinWriter, opcode.PACK)
	emit.String(script.BinWriter, "Put")
	emit.AppCall(script.BinWriter, hash.Hash160(avm), false)

	tx3 := transaction.NewInvocationTX(script.Bytes(), util.Fixed8FromFloat(100))
	b := bc.newBlock(newMinerTX(), tx3)
	require.NoError(t, bc.AddBlock(b))

	outStream, err := os.Create("../rpc/testdata/testblocks.acc")
	require.NoError(t, err)
	defer outStream.Close()

	writer := io.NewBinWriterFromIO(outStream)

	count := bc.BlockHeight() + 1
	writer.WriteU32LE(count - 1)

	for i := 1; i < int(count); i++ {
		bh := bc.GetHeaderHash(i)
		b, err := bc.GetBlock(bh)
		require.NoError(t, err)
		buf := io.NewBufBinWriter()
		b.EncodeBinary(buf.BinWriter)
		bytes := buf.Bytes()
		writer.WriteBytes(bytes)
		require.NoError(t, writer.Err)
	}
}
