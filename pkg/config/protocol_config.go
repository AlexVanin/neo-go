package config

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		Magic       netmode.Magic `yaml:"Magic"`
		MemPoolSize int           `yaml:"MemPoolSize"`
		// P2PNotaryRequestPayloadPoolSize specifies the memory pool size for P2PNotaryRequestPayloads.
		// It is valid only if P2PSigExtensions are enabled.
		P2PNotaryRequestPayloadPoolSize int `yaml:"P2PNotaryRequestPayloadPoolSize"`
		// KeepOnlyLatestState specifies if MPT should only store latest state.
		// If true, DB size will be smaller, but older roots won't be accessible.
		// This value should remain the same for the same database.
		KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
		// RemoveUntraceableBlocks specifies if old blocks should be removed.
		RemoveUntraceableBlocks bool `yaml:"RemoveUntraceableBlocks"`
		// MaxTraceableBlocks is the length of the chain accessible to smart contracts.
		MaxTraceableBlocks uint32 `yaml:"MaxTraceableBlocks"`
		// MaxTransactionsPerBlock is the maximum amount of transactions per block.
		MaxTransactionsPerBlock uint16 `yaml:"MaxTransactionsPerBlock"`
		// P2PSigExtensions enables additional signature-related logic.
		P2PSigExtensions bool `yaml:"P2PSigExtensions"`
		// ReservedAttributes allows to have reserved attributes range for experimental or private purposes.
		ReservedAttributes bool `yaml:"ReservedAttributes"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock  int      `yaml:"SecondsPerBlock"`
		SeedList         []string `yaml:"SeedList"`
		StandbyCommittee []string `yaml:"StandbyCommittee"`
		// StateRooInHeader enables storing state root in block header.
		StateRootInHeader bool `yaml:"StateRootInHeader"`
		ValidatorsCount   int  `yaml:"ValidatorsCount"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)
