package response

import (
	"encoding/json"

	"github.com/CityOfZion/neo-go/pkg/rpc/request"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// InvokeResult represents the outcome of a script that is
// executed by the NEO VM.
type InvokeResult struct {
	State       vm.State `json:"state"`
	GasConsumed string   `json:"gas_consumed"`
	Script      string   `json:"script"`
	Stack       []request.StackParam
}

// Header is a generic JSON-RPC 2.0 response header (ID and JSON-RPC version).
type Header struct {
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
}

// HeaderAndError adds an Error (that can be empty) to the Header, it's used
// to construct type-specific responses.
type HeaderAndError struct {
	Header
	Error *Error `json:"error,omitempty"`
}

// Raw represents a standard raw JSON-RPC 2.0
// response: http://www.jsonrpc.org/specification#response_object.
type Raw struct {
	HeaderAndError
	Result json.RawMessage `json:"result,omitempty"`
}

// GetTxOut represents result of `gettxout` RPC call.
type GetTxOut struct {
	HeaderAndError
	Result *result.TransactionOutput
}

// GetRawTx represents verbose output of `getrawtransaction` RPC call.
type GetRawTx struct {
	HeaderAndError
	Result *result.TransactionOutputRaw `json:"result"`
}
