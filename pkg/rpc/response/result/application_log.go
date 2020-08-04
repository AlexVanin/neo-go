package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ApplicationLog wrapper used for the representation of the
// state.AppExecResult based on the specific tx on the RPC Server.
type ApplicationLog struct {
	TxHash      util.Uint256              `json:"txid"`
	Trigger     string                    `json:"trigger"`
	VMState     string                    `json:"vmstate"`
	GasConsumed int64                     `json:"gasconsumed,string"`
	Stack       []smartcontract.Parameter `json:"stack"`
	Events      []NotificationEvent       `json:"notifications"`
}

//NotificationEvent response wrapper
type NotificationEvent struct {
	Contract util.Uint160            `json:"contract"`
	Name     string                  `json:"eventname"`
	Item     smartcontract.Parameter `json:"state"`
}

// StateEventToResultNotification converts state.NotificationEvent to
// result.NotificationEvent.
func StateEventToResultNotification(event state.NotificationEvent) NotificationEvent {
	seen := make(map[stackitem.Item]bool)
	item := smartcontract.ParameterFromStackItem(event.Item, seen)
	return NotificationEvent{
		Contract: event.ScriptHash,
		Name:     event.Name,
		Item:     item,
	}
}

// NewApplicationLog creates a new ApplicationLog wrapper.
func NewApplicationLog(appExecRes *state.AppExecResult) ApplicationLog {
	events := make([]NotificationEvent, 0, len(appExecRes.Events))
	for _, e := range appExecRes.Events {
		events = append(events, StateEventToResultNotification(e))
	}
	st := make([]smartcontract.Parameter, len(appExecRes.Stack))
	seen := make(map[stackitem.Item]bool)
	for i := range appExecRes.Stack {
		st[i] = smartcontract.ParameterFromStackItem(appExecRes.Stack[i], seen)
	}
	return ApplicationLog{
		TxHash:      appExecRes.TxHash,
		Trigger:     appExecRes.Trigger.String(),
		VMState:     appExecRes.VMState.String(),
		GasConsumed: appExecRes.GasConsumed,
		Stack:       st,
		Events:      events,
	}
}
