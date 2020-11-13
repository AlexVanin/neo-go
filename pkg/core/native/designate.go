package native

import (
	"encoding/binary"
	"errors"
	"math"
	"sort"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Designate represents designation contract.
type Designate struct {
	interop.ContractMD
	NEO *NEO

	rolesChangedFlag atomic.Value
	oracles          atomic.Value
}

type oraclesData struct {
	nodes  keys.PublicKeys
	addr   util.Uint160
	height uint32
}

const (
	designateContractID = -5
	designateName       = "Designation"

	// maxNodeCount is the maximum number of nodes to set the role for.
	maxNodeCount = 32
)

// Role represents type of participant.
type Role byte

// Role enumeration.
const (
	RoleStateValidator Role = 4
	RoleOracle         Role = 8
)

// Various errors.
var (
	ErrAlreadyDesignated = errors.New("already designated given role at current block")
	ErrEmptyNodeList     = errors.New("node list is empty")
	ErrInvalidIndex      = errors.New("invalid index")
	ErrInvalidRole       = errors.New("invalid role")
	ErrLargeNodeList     = errors.New("node list is too large")
	ErrNoBlock           = errors.New("no persisting block in the context")
)

func isValidRole(r Role) bool {
	return r == RoleOracle || r == RoleStateValidator
}

func newDesignate() *Designate {
	s := &Designate{ContractMD: *interop.NewContractMD(designateName)}
	s.ContractID = designateContractID

	desc := newDescriptor("getDesignatedByRole", smartcontract.ArrayType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("index", smartcontract.IntegerType))
	md := newMethodAndPrice(s.getDesignatedByRole, 1000000, smartcontract.AllowStates)
	s.AddMethod(md, desc, false)

	desc = newDescriptor("designateAsRole", smartcontract.VoidType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("nodes", smartcontract.ArrayType))
	md = newMethodAndPrice(s.designateAsRole, 0, smartcontract.AllowModifyStates)
	s.AddMethod(md, desc, false)

	desc = newDescriptor("name", smartcontract.StringType)
	md = newMethodAndPrice(nameMethod(designateName), 0, smartcontract.NoneFlag)
	s.AddMethod(md, desc, true)

	return s
}

// Initialize initializes Oracle contract.
func (s *Designate) Initialize(ic *interop.Context) error {
	return nil
}

// OnPersistEnd updates cached values if they've been changed.
func (s *Designate) OnPersistEnd(d dao.DAO) error {
	if !s.rolesChanged() {
		return nil
	}

	nodeKeys, height, err := s.GetDesignatedByRole(d, RoleOracle, math.MaxUint32)
	if err != nil {
		return err
	}

	od := &oraclesData{
		nodes:  nodeKeys,
		addr:   oracleHashFromNodes(nodeKeys),
		height: height,
	}
	s.oracles.Store(od)
	s.rolesChangedFlag.Store(false)
	return nil
}

// Metadata returns contract metadata.
func (s *Designate) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

func (s *Designate) getDesignatedByRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	ind, err := args[1].TryInteger()
	if err != nil || !ind.IsUint64() {
		panic(ErrInvalidIndex)
	}
	index := ind.Uint64()
	if index > uint64(ic.Chain.BlockHeight()+1) {
		panic(ErrInvalidIndex)
	}
	pubs, _, err := s.GetDesignatedByRole(ic.DAO, r, uint32(index))
	if err != nil {
		panic(err)
	}
	return pubsToArray(pubs)
}

func (s *Designate) rolesChanged() bool {
	rc := s.rolesChangedFlag.Load()
	return rc == nil || rc.(bool)
}

func oracleHashFromNodes(nodes keys.PublicKeys) util.Uint160 {
	if len(nodes) == 0 {
		return util.Uint160{}
	}
	script, _ := smartcontract.CreateMajorityMultiSigRedeemScript(nodes.Copy())
	return hash.Hash160(script)
}

func (s *Designate) getLastDesignatedHash(d dao.DAO, r Role) (util.Uint160, error) {
	if !isValidRole(r) {
		return util.Uint160{}, ErrInvalidRole
	}
	if r == RoleOracle && !s.rolesChanged() {
		odVal := s.oracles.Load()
		if odVal != nil {
			od := odVal.(*oraclesData)
			return od.addr, nil
		}
	}
	nodes, _, err := s.GetDesignatedByRole(d, r, math.MaxUint32)
	if err != nil {
		return util.Uint160{}, err
	}
	// We only have hashing defined for oracles now.
	return oracleHashFromNodes(nodes), nil
}

// GetDesignatedByRole returns nodes for role r.
func (s *Designate) GetDesignatedByRole(d dao.DAO, r Role, index uint32) (keys.PublicKeys, uint32, error) {
	if !isValidRole(r) {
		return nil, 0, ErrInvalidRole
	}
	if r == RoleOracle && !s.rolesChanged() {
		odVal := s.oracles.Load()
		if odVal != nil {
			od := odVal.(*oraclesData)
			if od.height <= index {
				return od.nodes, od.height, nil
			}
		}
	}
	kvs, err := d.GetStorageItemsWithPrefix(s.ContractID, []byte{byte(r)})
	if err != nil {
		return nil, 0, err
	}
	var ns NodeList
	var bestIndex uint32
	var resSi *state.StorageItem
	for k, si := range kvs {
		if len(k) < 4 {
			continue
		}
		siInd := binary.BigEndian.Uint32([]byte(k))
		if (resSi == nil || siInd > bestIndex) && siInd <= index {
			bestIndex = siInd
			resSi = si
		}
	}
	if resSi != nil {
		reader := io.NewBinReaderFromBuf(resSi.Value)
		ns.DecodeBinary(reader)
		if reader.Err != nil {
			return nil, 0, reader.Err
		}
	}
	return keys.PublicKeys(ns), bestIndex, err
}

func (s *Designate) designateAsRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	var ns NodeList
	if err := ns.fromStackItem(args[1]); err != nil {
		panic(err)
	}

	err := s.DesignateAsRole(ic, r, keys.PublicKeys(ns))
	if err != nil {
		panic(err)
	}
	return pubsToArray(keys.PublicKeys(ns))
}

// DesignateAsRole sets nodes for role r.
func (s *Designate) DesignateAsRole(ic *interop.Context, r Role, pubs keys.PublicKeys) error {
	length := len(pubs)
	if length == 0 {
		return ErrEmptyNodeList
	}
	if length > maxNodeCount {
		return ErrLargeNodeList
	}
	if !isValidRole(r) {
		return ErrInvalidRole
	}
	h := s.NEO.GetCommitteeAddress()
	if ok, err := runtime.CheckHashedWitness(ic, h); err != nil || !ok {
		return ErrInvalidWitness
	}
	if ic.Block == nil {
		return ErrNoBlock
	}
	var key = make([]byte, 5)
	key[0] = byte(r)
	binary.BigEndian.PutUint32(key[1:], ic.Block.Index+1)

	si := ic.DAO.GetStorageItem(s.ContractID, key)
	if si != nil {
		return ErrAlreadyDesignated
	}
	sort.Sort(pubs)
	s.rolesChangedFlag.Store(true)
	si = &state.StorageItem{Value: NodeList(pubs).Bytes()}
	return ic.DAO.PutStorageItem(s.ContractID, key, si)
}

func getRole(item stackitem.Item) (Role, bool) {
	bi, err := item.TryInteger()
	if err != nil {
		return 0, false
	}
	if !bi.IsUint64() {
		return 0, false
	}
	u := bi.Uint64()
	return Role(u), u <= math.MaxUint8 && isValidRole(Role(u))
}
