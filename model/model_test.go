package model

import (
	"github.com/stretchr/testify/require"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	document := &Document{Structs: NamedList[*StructDescriptor]{
		{
			Name:        "Thing",
			Description: "Represents a thing.",
			Fields: NamedList[*FieldDescriptor]{
				{Name: "id", Description: "The stable ID."},
				{Name: "count", Type: &TypeDescriptor{Kind: TypeDescriptorKindBasic, Name: "integer"}},
			},
		},
	}}

	g := NewGenerator(document)
	node, err := g.Generate()
	require.NoError(t, err)

	buff := strings.Builder{}
	fset := token.NewFileSet()
	err = printer.Fprint(&buff, fset, node)
	require.NoError(t, err)

	require.NotEmpty(t, buff.String())
	require.Contains(t, buff.String(), "// Thing: Represents a thing.")
	require.Contains(t, buff.String(), "// Id: The stable ID.")
}
