package runtime

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

const (
	// MaxEventNameLen is the maximum length of a name for event.
	MaxEventNameLen = 32
	// MaxNotificationSize is the maximum length of a runtime log message.
	MaxNotificationSize = 1024
)

// GetExecutingScriptHash returns executing script hash.
func GetExecutingScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(0)
}

// GetCallingScriptHash returns calling script hash.
func GetCallingScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(1)
}

// GetEntryScriptHash returns entry script hash.
func GetEntryScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(ic.VM.Istack().Len() - 1)
}

// Platform returns the name of the platform.
func Platform(ic *interop.Context) error {
	ic.VM.Estack().PushVal([]byte("NEO"))
	return nil
}

// GetTrigger returns the script trigger.
func GetTrigger(ic *interop.Context) error {
	ic.VM.Estack().PushVal(byte(ic.Trigger))
	return nil
}

// Notify should pass stack item to the notify plugin to handle it, but
// in neo-go the only meaningful thing to do here is to log.
func Notify(ic *interop.Context) error {
	name := ic.VM.Estack().Pop().String()
	if len(name) > MaxEventNameLen {
		return fmt.Errorf("event name must be less than %d", MaxEventNameLen)
	}
	elem := ic.VM.Estack().Pop()
	args := elem.Array()
	// But it has to be serializable, otherwise we either have some broken
	// (recursive) structure inside or an interop item that can't be used
	// outside of the interop subsystem anyway.
	bytes, err := stackitem.SerializeItem(elem.Item())
	if err != nil {
		return fmt.Errorf("bad notification: %w", err)
	}
	if len(bytes) > MaxNotificationSize {
		return fmt.Errorf("notification size shouldn't exceed %d", MaxNotificationSize)
	}
	ne := state.NotificationEvent{
		ScriptHash: ic.VM.GetCurrentScriptHash(),
		Name:       name,
		Item:       stackitem.DeepCopy(stackitem.NewArray(args)).(*stackitem.Array),
	}
	ic.Notifications = append(ic.Notifications, ne)
	return nil
}

// Log logs the message passed.
func Log(ic *interop.Context) error {
	state := ic.VM.Estack().Pop().String()
	if len(state) > MaxNotificationSize {
		return fmt.Errorf("message length shouldn't exceed %v", MaxNotificationSize)
	}
	msg := fmt.Sprintf("%q", state)
	ic.Log.Info("runtime log",
		zap.Stringer("script", ic.VM.GetCurrentScriptHash()),
		zap.String("logs", msg))
	return nil
}

// GetTime returns timestamp of the block being verified, or the latest
// one in the blockchain if no block is given to Context.
func GetTime(ic *interop.Context) error {
	header := ic.Block.Header()
	ic.VM.Estack().PushVal(header.Timestamp)
	return nil
}
