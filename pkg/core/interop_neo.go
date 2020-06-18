package core

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxContractDescriptionLen is the maximum length for contract description.
	MaxContractDescriptionLen = 65536
	// MaxContractScriptSize is the maximum script size for a contract.
	MaxContractScriptSize = 1024 * 1024
	// MaxContractParametersNum is the maximum number of parameters for a contract.
	MaxContractParametersNum = 252
	// MaxContractStringLen is the maximum length for contract metadata strings.
	MaxContractStringLen = 252
)

var errGasLimitExceeded = errors.New("gas limit exceeded")

// storageFind finds stored key-value pair.
func storageFind(ic *interop.Context, v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	err := checkStorageContext(ic, stc)
	if err != nil {
		return err
	}
	prefix := v.Estack().Pop().Bytes()
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(stc.ScriptHash, prefix)
	if err != nil {
		return err
	}

	filteredMap := stackitem.NewMap()
	for k, v := range siMap {
		filteredMap.Add(stackitem.NewByteArray(append(prefix, []byte(k)...)), stackitem.NewByteArray(v.Value))
	}
	sort.Slice(filteredMap.Value().([]stackitem.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]stackitem.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]stackitem.MapElement)[j].Key.Value().([]byte)) == -1
	})

	item := vm.NewMapIterator(filteredMap)
	v.Estack().PushVal(item)

	return nil
}

// createContractStateFromVM pops all contract state elements from the VM
// evaluation stack, does a lot of checks and returns Contract if it
// succeeds.
func createContractStateFromVM(ic *interop.Context, v *vm.VM) (*state.Contract, error) {
	script := v.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return nil, errors.New("the script is too big")
	}
	manifestBytes := v.Estack().Pop().Bytes()
	if len(manifestBytes) > manifest.MaxManifestSize {
		return nil, errors.New("manifest is too big")
	}
	if !v.AddGas(util.Fixed8(StoragePrice * (len(script) + len(manifestBytes)))) {
		return nil, errGasLimitExceeded
	}
	var m manifest.Manifest
	err := m.UnmarshalJSON(manifestBytes)
	if err != nil {
		return nil, err
	}
	return &state.Contract{
		Script:   script,
		Manifest: m,
	}, nil
}

// contractCreate creates a contract.
func contractCreate(ic *interop.Context, v *vm.VM) error {
	newcontract, err := createContractStateFromVM(ic, v)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(newcontract.ScriptHash())
	if contract != nil {
		return errors.New("contract already exists")
	}
	id, err := ic.DAO.GetNextContractID()
	if err != nil {
		return err
	}
	newcontract.ID = id
	if err := ic.DAO.PutNextContractID(id); err != nil {
		return err
	}
	if err := ic.DAO.PutContractState(newcontract); err != nil {
		return err
	}
	v.Estack().PushVal(stackitem.NewInterop(newcontract))
	return nil
}

// contractUpdate migrates a contract.
func contractUpdate(ic *interop.Context, v *vm.VM) error {
	contract, err := ic.DAO.GetContractState(v.GetCurrentScriptHash())
	if contract == nil {
		return errors.New("contract doesn't exist")
	}
	newcontract, err := createContractStateFromVM(ic, v)
	if err != nil {
		return err
	}
	if newcontract.Script != nil {
		if l := len(newcontract.Script); l == 0 || l > MaxContractScriptSize {
			return errors.New("invalid script len")
		}
		h := newcontract.ScriptHash()
		if h.Equals(contract.ScriptHash()) {
			return errors.New("the script is the same")
		} else if _, err := ic.DAO.GetContractState(h); err == nil {
			return errors.New("contract already exists")
		}
		newcontract.ID = contract.ID
		if err := ic.DAO.PutContractState(newcontract); err != nil {
			return err
		}
		if err := ic.DAO.DeleteContractState(contract.ScriptHash()); err != nil {
			return err
		}
	}
	if contract.HasStorage() {
		// TODO store items by ID #1037
		hash := v.GetCurrentScriptHash()
		siMap, err := ic.DAO.GetStorageItems(hash)
		if err != nil {
			return err
		}
		for k, v := range siMap {
			v.IsConst = false
			err = ic.DAO.PutStorageItem(contract.ScriptHash(), []byte(k), v)
			if err != nil {
				return err
			}
		}
	}
	v.Estack().PushVal(stackitem.NewInterop(contract))
	return contractDestroy(ic, v)
}

// runtimeSerialize serializes top stack item into a ByteArray.
func runtimeSerialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeSerialize(v)
}

// runtimeDeserialize deserializes ByteArray from a stack into an item.
func runtimeDeserialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeDeserialize(v)
}
