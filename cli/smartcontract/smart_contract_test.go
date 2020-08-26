package smartcontract

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/require"
)

func TestParseParams_CalledFromItself(t *testing.T) {
	testCases := map[string]struct {
		WordsRead int
		Value     []smartcontract.Parameter
	}{
		"]": {
			WordsRead: 1,
			Value:     []smartcontract.Parameter{},
		},
		"[ [ ] ] ]": {
			WordsRead: 5,
			Value: []smartcontract.Parameter{
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.ArrayType,
							Value: []smartcontract.Parameter{},
						},
					},
				},
			},
		},
		"a b c ]": {
			WordsRead: 4,
			Value: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type:  smartcontract.StringType,
					Value: "b",
				},
				{
					Type:  smartcontract.StringType,
					Value: "c",
				},
			},
		},
		"a [ b [ [ c d ] e ] ] f ] extra items": {
			WordsRead: 13, // the method should return right after the last bracket, as calledFromMain == false
			Value: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.StringType,
							Value: "b",
						},
						{
							Type: smartcontract.ArrayType,
							Value: []smartcontract.Parameter{
								{
									Type: smartcontract.ArrayType,
									Value: []smartcontract.Parameter{
										{
											Type:  smartcontract.StringType,
											Value: "c",
										},
										{
											Type:  smartcontract.StringType,
											Value: "d",
										},
									},
								},
								{
									Type:  smartcontract.StringType,
									Value: "e",
								},
							},
						},
					},
				},
				{
					Type:  smartcontract.StringType,
					Value: "f",
				},
			},
		},
	}

	for str, expected := range testCases {
		input := strings.Split(str, " ")
		offset, actual, err := parseParams(input, false)
		require.NoError(t, err)
		require.Equal(t, expected.WordsRead, offset)
		require.Equal(t, expected.Value, actual)
	}

	errorCases := []string{
		"[ ]",
		"[ a b [ c ] d ]",
		"[ ] --",
		"--",
		"not-int:integer ]",
	}

	for _, str := range errorCases {
		input := strings.Split(str, " ")
		_, _, err := parseParams(input, false)
		require.Error(t, err)
	}
}

func TestParseParams_CalledFromOutside(t *testing.T) {
	testCases := map[string]struct {
		WordsRead  int
		Parameters []smartcontract.Parameter
	}{
		"-- cosigner1": {
			WordsRead:  1, // the `--` only
			Parameters: []smartcontract.Parameter{},
		},
		"a b c": {
			WordsRead: 3,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type:  smartcontract.StringType,
					Value: "b",
				},
				{
					Type:  smartcontract.StringType,
					Value: "c",
				},
			},
		},
		"a b c -- cosigner1": {
			WordsRead: 4,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type:  smartcontract.StringType,
					Value: "b",
				},
				{
					Type:  smartcontract.StringType,
					Value: "c",
				},
			},
		},
		"a [ b [ [ c d ] e ] ] f": {
			WordsRead: 12,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.StringType,
							Value: "b",
						},
						{
							Type: smartcontract.ArrayType,
							Value: []smartcontract.Parameter{
								{
									Type: smartcontract.ArrayType,
									Value: []smartcontract.Parameter{
										{
											Type:  smartcontract.StringType,
											Value: "c",
										},
										{
											Type:  smartcontract.StringType,
											Value: "d",
										},
									},
								},
								{
									Type:  smartcontract.StringType,
									Value: "e",
								},
							},
						},
					},
				},
				{
					Type:  smartcontract.StringType,
					Value: "f",
				},
			},
		},
		"a [ b ] -- cosigner1 cosigner2": {
			WordsRead: 5,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.StringType,
							Value: "b",
						},
					},
				},
			},
		},
		"a [ b ]": {
			WordsRead: 4,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.StringType,
							Value: "b",
						},
					},
				},
			},
		},
		"a [ b ] [ [ c ] ] [ [ [ d ] ] ]": {
			WordsRead: 16,
			Parameters: []smartcontract.Parameter{
				{
					Type:  smartcontract.StringType,
					Value: "a",
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type:  smartcontract.StringType,
							Value: "b",
						},
					},
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type: smartcontract.ArrayType,
							Value: []smartcontract.Parameter{
								{
									Type:  smartcontract.StringType,
									Value: "c",
								},
							},
						},
					},
				},
				{
					Type: smartcontract.ArrayType,
					Value: []smartcontract.Parameter{
						{
							Type: smartcontract.ArrayType,
							Value: []smartcontract.Parameter{
								{
									Type: smartcontract.ArrayType,
									Value: []smartcontract.Parameter{
										{
											Type:  smartcontract.StringType,
											Value: "d",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for str, expected := range testCases {
		input := strings.Split(str, " ")
		offset, arr, err := parseParams(input, true)
		require.NoError(t, err)
		require.Equal(t, expected.WordsRead, offset)
		require.Equal(t, expected.Parameters, arr)
	}

	errorCases := []string{
		"[",
		"]",
		"[ [ ]",
		"[ [ ] --",
		"[ -- ]",
	}
	for _, str := range errorCases {
		input := strings.Split(str, " ")
		_, _, err := parseParams(input, true)
		require.Error(t, err)
	}
}
