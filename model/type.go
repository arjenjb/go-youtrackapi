package model

import (
	"go/ast"
)

type TypeKind uint8

const (
	TypeUnknown TypeKind = iota
	TypeBuiltin
	TypeInterface
	TypeStruct
	TypePointer
	TypeSlice
	TypeMap
)

type Type struct {
	Name    string
	Package string
	Kind    TypeKind
	Elem    *Type
}

func (t Type) String() string {
	switch t.Kind {
	case TypePointer:
		return "*" + t.Elem.String()
	case TypeSlice:
		return "[]" + t.Elem.String()
	case TypeStruct:
		result := ""
		if t.Package != "" {
			result += t.Package + "."
		}
		result += t.Name
		return result
	case TypeBuiltin:
		return t.Name
	case TypeInterface:
		return t.Name
	default:
		panic("should not occur")
	}
}

func (t Type) Ptr() Type {
	return Type{
		Kind: TypePointer,
		Elem: &t,
	}
}

func (t Type) Ast() ast.Expr {
	switch t.Kind {
	case TypePointer:
		return &ast.StarExpr{
			X: t.Elem.Ast(),
		}
	case TypeSlice:
		return &ast.ArrayType{
			Elt: t.Elem.Ast(),
		}
	case TypeStruct, TypeBuiltin, TypeInterface:
		return t.QualifiedName()

	default:
		panic("should not occur")
	}
}

func (t Type) QualifiedName() ast.Expr {
	if len(t.Package) == 0 {
		return ast.NewIdent(t.Name)
	} else {
		return &ast.SelectorExpr{
			X:   ast.NewIdent(t.Package),
			Sel: ast.NewIdent(t.Name),
		}
	}
}

func (t Type) Slice() Type {
	return Type{
		Kind: TypeSlice,
		Elem: &t,
	}
}
