package bst

import (
	"github.com/stretchr/testify/require"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestUnParse(t *testing.T) {
	f := ast.File{
		Name: ast.NewIdent("foo"),
		Decls: []ast.Decl{
			&ast.GenDecl{
				Tok: token.VAR,
				Specs: []ast.Spec{
					&ast.ValueSpec{
						Names: []*ast.Ident{ast.NewIdent("foo")},
						Type:  ast.NewIdent("string"),
					},
				},
			},
		},
	}

	buff := strings.Builder{}
	fset := token.NewFileSet()
	err := printer.Fprint(&buff, fset, &f)
	require.NoError(t, err)

	t.Log(buff.String())
}
