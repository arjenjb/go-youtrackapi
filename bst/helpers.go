package bst

import (
	"fmt"
	"go/ast"
	"go/token"
)

//func BinaryExpr(left ExprNode, op string, right ExprNode) BinaryExprNode {
//	return BinaryExprNode{
//		Left:  left,
//		Op:    op,
//		Right: right,
//	}
//}

func Ident(s string) *ast.Ident {
	return &ast.Ident{Name: s}
}

//func Deref(node ExprNode) DerefNode {
//	return DerefNode{Value: node}
//}

func Struct(ident *ast.Ident, fields []*ast.Field) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ident,
				Type: &ast.StructType{
					Fields: &ast.FieldList{List: fields},
				},
			},
		},
	}
}

func Field(name *ast.Ident, t ast.Expr) *ast.Field {
	var names []*ast.Ident
	if name != nil {
		names = append(names, name)
	}

	return &ast.Field{
		Names: names,
		Type:  t,
	}
}

func FieldList(fields ...*ast.Field) *ast.FieldList {
	return &ast.FieldList{List: fields}
}

func Declare(x *ast.Ident, t *ast.Ident) ast.Stmt {
	return &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{x},
					Type:  t,
				},
			},
		},
	}
}

func Define(name ast.Expr, val ast.Expr) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{name},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{val},
	}
}

func Assign(name ast.Expr, val ast.Expr) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{name},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{val},
	}
}

func MultiDefine(lhs []ast.Expr, rhs []ast.Expr) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: lhs,
		Tok: token.DEFINE,
		Rhs: rhs,
	}
}

func MultiAssign(lhs []ast.Expr, rhs []ast.Expr) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: lhs,
		Tok: token.ASSIGN,
		Rhs: rhs,
	}
}

func Address(x ast.Expr) ast.Expr {
	return &ast.UnaryExpr{
		Op: token.AND,
		X:  x,
	}
}

func Composite(x ast.Expr) ast.Expr {
	return &ast.CompositeLit{
		Type: x,
		Elts: nil,
	}
}

func Call(fun ast.Expr, args ...ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun:  fun,
		Args: args,
	}
}

func Select(x *ast.Ident, sel *ast.Ident) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		X:   x,
		Sel: sel,
	}
}

func Return(results ...ast.Expr) ast.Stmt {
	return &ast.ReturnStmt{
		Results: results,
	}
}

func Block(stmts ...ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{
		List: stmts,
	}
}

func String(name string) *ast.BasicLit {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("%#v", name),
	}
}

func NotNil(x ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:  x,
		Op: token.NEQ,
		Y:  Nil,
	}
}

func If(cond ast.Expr, block *ast.BlockStmt) ast.Stmt {
	return &ast.IfStmt{
		Cond: cond,
		Body: block,
	}
}

func Ptr(x ast.Expr) ast.Expr {
	return &ast.StarExpr{
		X: x,
	}
}

func Stmts(stmts ...ast.Stmt) []ast.Stmt {
	return stmts
}

func Exprs(expr ...ast.Expr) []ast.Expr {
	return expr
}
