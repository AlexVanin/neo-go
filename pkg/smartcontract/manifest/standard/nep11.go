package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

var nep11Base = &Standard{
	Base: decimalTokenBase,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name:       "tokensOf",
					ReturnType: smartcontract.AnyType, // Iterator
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
					},
					Safe: true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.BoolType,
				},
			},
			Events: []manifest.Event{
				{
					Name: "Transfer",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.IntegerType},
						{Type: smartcontract.ByteArrayType},
					},
				},
			},
		},
	},
	Optional: []manifest.Method{
		{
			Name: "properties",
			Parameters: []manifest.Parameter{
				{Type: smartcontract.ByteArrayType},
			},
			ReturnType: smartcontract.MapType,
			Safe:       true,
		},
		{
			Name:       "tokens",
			ReturnType: smartcontract.AnyType,
			Safe:       true,
		},
	},
}

var nep11NonDivisible = &Standard{
	Base: nep11Base,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "ownerOf",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.Hash160Type,
					Safe:       true,
				},
			},
		},
	},
}

var nep11Divisible = &Standard{
	Base: nep11Base,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name: "ownerOf",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.AnyType,
					Safe:       true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.IntegerType},
						{Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.BoolType,
				},
			},
		},
	},
}
