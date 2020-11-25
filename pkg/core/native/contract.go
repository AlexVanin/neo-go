package native

import (
	"errors"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// reservedContractID represents the upper bound of the reserved IDs for native contracts.
const reservedContractID = -100

// Contracts is a set of registered native contracts.
type Contracts struct {
	NEO       *NEO
	GAS       *GAS
	Policy    *Policy
	Oracle    *Oracle
	Designate *Designate
	Notary    *Notary
	Contracts []interop.Contract
	// persistScript is vm script which executes "onPersist" method of every native contract.
	persistScript []byte
	// postPersistScript is vm script which executes "postPersist" method of every native contract.
	postPersistScript []byte
}

// ByHash returns native contract with the specified hash.
func (cs *Contracts) ByHash(h util.Uint160) interop.Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().Hash.Equals(h) {
			return ctr
		}
	}
	return nil
}

// ByName returns native contract with the specified name.
func (cs *Contracts) ByName(name string) interop.Contract {
	name = strings.ToLower(name)
	for _, ctr := range cs.Contracts {
		if strings.ToLower(ctr.Metadata().Name) == name {
			return ctr
		}
	}
	return nil
}

// NewContracts returns new set of native contracts with new GAS, NEO, Policy, Oracle,
// Designate and (optional) Notary contracts.
func NewContracts(p2pSigExtensionsEnabled bool) *Contracts {
	cs := new(Contracts)

	gas := newGAS()
	neo := newNEO()
	neo.GAS = gas
	gas.NEO = neo

	cs.GAS = gas
	cs.Contracts = append(cs.Contracts, gas)
	cs.NEO = neo
	cs.Contracts = append(cs.Contracts, neo)

	policy := newPolicy()
	cs.Policy = policy
	cs.Contracts = append(cs.Contracts, policy)

	oracle := newOracle()
	oracle.GAS = gas
	oracle.NEO = neo
	cs.Oracle = oracle
	cs.Contracts = append(cs.Contracts, oracle)

	desig := newDesignate(p2pSigExtensionsEnabled)
	desig.NEO = neo
	cs.Designate = desig
	cs.Oracle.Desig = desig
	cs.Contracts = append(cs.Contracts, desig)

	if p2pSigExtensionsEnabled {
		notary := newNotary()
		notary.GAS = gas
		notary.Desig = desig
		cs.Notary = notary
		cs.Contracts = append(cs.Contracts, notary)
	}

	return cs
}

// GetPersistScript returns VM script calling "onPersist" method of every native contract.
func (cs *Contracts) GetPersistScript() []byte {
	if cs.persistScript != nil {
		return cs.persistScript
	}
	w := io.NewBufBinWriter()
	for i := range cs.Contracts {
		md := cs.Contracts[i].Metadata()
		// Not every contract is persisted:
		// https://github.com/neo-project/neo/blob/master/src/neo/Ledger/Blockchain.cs#L90
		if md.ContractID == policyContractID || md.ContractID == oracleContractID || md.ContractID == designateContractID {
			continue
		}
		emit.Int(w.BinWriter, 0)
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY)
		emit.String(w.BinWriter, "onPersist")
		emit.AppCall(w.BinWriter, md.Hash)
		emit.Opcodes(w.BinWriter, opcode.DROP)
	}
	cs.persistScript = w.Bytes()
	return cs.persistScript
}

// GetPostPersistScript returns VM script calling "postPersist" method of some native contracts.
func (cs *Contracts) GetPostPersistScript() []byte {
	if cs.postPersistScript != nil {
		return cs.postPersistScript
	}
	w := io.NewBufBinWriter()
	for i := range cs.Contracts {
		md := cs.Contracts[i].Metadata()
		// Not every contract is persisted:
		// https://github.com/neo-project/neo/blob/master/src/neo/Ledger/Blockchain.cs#L103
		if md.ContractID == policyContractID || md.ContractID == gasContractID || md.ContractID == designateContractID || md.ContractID == notaryContractID {
			continue
		}
		emit.Int(w.BinWriter, 0)
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY)
		emit.String(w.BinWriter, "postPersist")
		emit.AppCall(w.BinWriter, md.Hash)
		emit.Opcodes(w.BinWriter, opcode.DROP)
	}
	cs.postPersistScript = w.Bytes()
	return cs.postPersistScript
}

func postPersistBase(ic *interop.Context) error {
	if ic.Trigger != trigger.PostPersist {
		return errors.New("postPersist must be trigered by system")
	}
	return nil
}

func onPersistBase(ic *interop.Context) error {
	if ic.Trigger != trigger.OnPersist {
		return errors.New("onPersist must be trigered by system")
	}
	return nil
}
