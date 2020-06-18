package config

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		// FeePerExtraByte sets the expected per-byte fee for
		// transactions exceeding the MaxFreeTransactionSize.
		FeePerExtraByte float64 `yaml:"FeePerExtraByte"`
		// FreeGasLimit is an amount of GAS which can be spent for free.
		FreeGasLimit            util.Fixed8   `yaml:"FreeGasLimit"`
		LowPriorityThreshold    float64       `yaml:"LowPriorityThreshold"`
		Magic                   netmode.Magic `yaml:"Magic"`
		MaxTransactionsPerBlock int           `yaml:"MaxTransactionsPerBlock"`
		// Maximum size of low priority transaction in bytes.
		MaxFreeTransactionSize int `yaml:"MaxFreeTransactionSize"`
		// Maximum number of low priority transactions accepted into block.
		MaxFreeTransactionsPerBlock int `yaml:"MaxFreeTransactionsPerBlock"`
		MemPoolSize                 int `yaml:"MemPoolSize"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch  bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock   int      `yaml:"SecondsPerBlock"`
		SeedList          []string `yaml:"SeedList"`
		StandbyValidators []string `yaml:"StandbyValidators"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)
