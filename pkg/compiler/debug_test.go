package compiler

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGen_DebugInfo(t *testing.T) {
	src := `package foo
func Main(op string) bool {
	res := methodInt(op)
	_ = methodString()	
	_ = methodByteArray()
	_ = methodArray()
	_ = methodStruct()
	return res == 42
}

func methodInt(a string) int {
	if a == "get42" {
		return 42
	}
	return 3
}
func methodString() string { return "" }
func methodByteArray() []byte { return nil }
func methodArray() []bool { return nil }
func methodStruct() struct{} { return struct{}{} }
`

	info, err := getBuildInfo(src)
	require.NoError(t, err)

	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)
	require.NoError(t, c.compile(info, pkg))

	buf := c.prog.Bytes()
	d := c.emitDebugInfo()
	require.NotNil(t, d)

	t.Run("return types", func(t *testing.T) {
		returnTypes := map[string]string{
			"methodInt":    "Integer",
			"methodString": "String", "methodByteArray": "ByteArray",
			"methodArray": "Array", "methodStruct": "Struct",
			"Main": "Boolean",
		}
		for i := range d.Methods {
			name := d.Methods[i].Name.Name
			assert.Equal(t, returnTypes[name], d.Methods[i].ReturnType)
		}
	})

	// basic check that last instruction of every method is indeed RET
	for i := range d.Methods {
		index := d.Methods[i].Range.End
		require.True(t, int(index) < len(buf))
		require.EqualValues(t, opcode.RET, buf[index])
	}
}

func TestDebugInfo_MarshalJSON(t *testing.T) {
	d := &DebugInfo{
		EntryPoint: "main",
		Documents:  []string{"/path/to/file"},
		Methods: []MethodDebugInfo{
			{
				ID: "id1",
				Name: DebugMethodName{
					Namespace: "default",
					Name:      "method1",
				},
				Range: DebugRange{Start: 10, End: 20},
				Parameters: []DebugParam{
					{"param1", "Integer"},
					{"ok", "Boolean"},
				},
				ReturnType: "ByteArray",
				Variables:  []string{},
				SeqPoints: []DebugSeqPoint{
					{
						Opcode:    123,
						Document:  1,
						StartLine: 2,
						StartCol:  3,
						EndLine:   4,
						EndCol:    5,
					},
				},
			},
		},
		Events: []EventDebugInfo{},
	}

	testserdes.MarshalUnmarshalJSON(t, d, new(DebugInfo))
}
