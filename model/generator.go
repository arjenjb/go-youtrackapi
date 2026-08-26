package model

import (
	"fmt"
	. "github.com/arjenjb/go-youtrackapi/bst"
	"go/ast"
	"go/token"
	"strings"
)

// todo: deal with enum types
// todo: marshalers for abstract types

var ErrorType = Type{Name: "error", Kind: TypeBuiltin}

type fieldTypePair struct {
	field *FieldDescriptor
	type_ *StructDescriptor
}

type fieldTypeCollection struct {
	order  map[string]int
	fields []fieldTypePair
}

func (n *fieldTypeCollection) add(pair fieldTypePair) {
	i, ok := n.order[pair.field.Name]
	if !ok {
		n.fields = append(n.fields, pair)
		n.order[pair.field.Name] = len(n.fields) - 1
	} else {
		n.fields[i] = pair
	}
}

type Generator struct {
	document *Document
	typeMap  map[string]Type
	nodes    []ast.Decl
}

func NewGenerator(doc *Document) *Generator {
	g := Generator{
		typeMap:  make(map[string]Type),
		document: doc,
	}

	g.typeMap["string"] = Type{Kind: TypeBuiltin, Name: "string"}
	g.typeMap["integer"] = Type{Kind: TypeBuiltin, Name: "int"}
	g.typeMap["boolean"] = Type{Kind: TypeBuiltin, Name: "bool"}
	g.typeMap["float"] = Type{Kind: TypeBuiltin, Name: "float64"}
	g.typeMap["any"] = Type{Kind: TypeBuiltin, Name: "any"}
	g.typeMap["Time"] = Type{Kind: TypeStruct, Name: "Time", Package: "time"}

	return &g
}

func (g Generator) Generate() (*ast.File, error) {
	g.nodes = append(g.nodes,
		&ast.GenDecl{
			Tok: token.IMPORT,
			Specs: []ast.Spec{
				&ast.ImportSpec{Path: String("fmt")},
				&ast.ImportSpec{Path: String("time")},
				&ast.ImportSpec{Path: String("log/slog")},
			},
		})
	// Generate interfaces
	for _, each := range g.document.Structs {
		if !each.Abstract {
			continue
		}
		err := g.generateInterfaceType(each)
		if err != nil {
			return nil, err
		}

		err = g.generateInterfaceUnmarshaller(each, g.document.AllChildrenOf(*each))
		if err != nil {
			return nil, err
		}

		err = g.generateInterfaceMarshaller(each, g.document.AllChildrenOf(*each))
		if err != nil {
			return nil, err
		}
	}

	// Generate the structs
	for _, each := range g.document.Structs {
		if each.Abstract {
			continue
		}
		err := g.generateStructType(each)
		if err != nil {
			return nil, err
		}

		err = g.generateStructUnmarshaler(each)
		if err != nil {
			return nil, err
		}

		err = g.generateStructMarshaler(each)
		if err != nil {
			return nil, err
		}
	}

	return &ast.File{
		Name:  Ident("youtrackapi"),
		Decls: g.nodes,
	}, nil
}

func (g *Generator) generateStructType(each *StructDescriptor) error {
	fields, err := g.generateStructFieldsOn(each)
	if err != nil {
		return err
	}

	var embeds []Type

	parent := Find(g.document.AllParentsOf(*each), func(each StructDescriptor) bool { return each.Abstract })
	if parent != nil {
		embeds = append(embeds, g.lookupType(parent.Name))
	}

	s := Struct(Ident(each.Name), fields)
	s.Doc = docComment(each.Name, each.Description)

	g.addStatement(s)
	return nil
}

func (g *Generator) generateStructUnmarshaler(each *StructDescriptor) error {
	name := fmt.Sprintf("unmarshal%s", each.Name)
	t := Type{Name: each.Name, Kind: TypeStruct}

	key := Ident("key")
	result := ast.NewIdent("result")
	errVar := ast.NewIdent("err")
	r := ast.NewIdent("r")

	var cases []ast.Stmt

	fields, e := g.collectStructFields(each)
	if e != nil {
		return e
	}

	for _, each := range fields {
		// figure out the type
		c, err := g.generateFieldUnmarshaler(each.field)
		if err != nil {
			return err
		}

		cases = append(cases, c)
	}

	val := Ident("val")
	cases = append(cases,
		&ast.CaseClause{
			List: Exprs(String("$type")),
			Body: Stmts(
				MultiAssign(
					Exprs(Ident("_"), Ident("_")),
					Exprs(Call(Select(Ident("r"), Ident("NextValue")))),
				),
			),
		},
		&ast.CaseClause{
			Body: []ast.Stmt{
				MultiDefine(
					Exprs(val, Ident("_")),
					Exprs(Call(Select(Ident("r"), Ident("NextValue")))),
				),
				&ast.ExprStmt{
					X: Call(
						Select(ast.NewIdent("slog"), ast.NewIdent("Warn")),
						String("Unsupported field"),
						String("key"),
						key,
						String("value"),
						val,
					),
				},
			}})

	var statements []ast.Stmt

	tokVar := Ident("tok")
	statements = append(
		statements,
		MultiDefine(Exprs(tokVar, errVar), Exprs(Call(Select(r, Ident("Peek"))))),
		If(NotNil(errVar), Block(
			Return(Nil, errVar))),
		If(Call(Select(tokVar, Ident("IsNull"))), Block(
			&ast.ExprStmt{
				X: Call(Select(r, Ident("Next"))),
			},
			Return(Nil, Nil))),
	)

	statements = append(
		statements,
		Define(result, Address(Composite(t.Ast()))),
		Assign(errVar, Call(
			Select(r, ast.NewIdent("NextObjectDo")),
			&ast.FuncLit{
				Type: &ast.FuncType{
					Params: FieldList(
						Field(key, Ident("string")),
						Field(r, Type{Name: "JSONReader", Kind: TypeBuiltin}.Ptr().Ast()),
					),
					Results: FieldList(Field(nil, Ident("error"))),
				},
				Body: Block(
					Declare(Ident("err"), Ident("error")),
					&ast.SwitchStmt{
						Tag: key,
						Body: Block(
							cases...,
						),
					},
					Return(Ident("err")),
				),
			},
		)),
		&ast.IfStmt{
			Cond: NotNil(errVar),
			Body: Block(
				Return(Nil, errVar),
			),
		},
		Return(result, Nil),
	)

	n := &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{
			Func:       0,
			TypeParams: nil,
			Params: FieldList(
				Field(ast.NewIdent("r"), Type{Name: "JSONReader", Kind: TypeStruct}.Ptr().Ast())),
			Results: FieldList(
				Field(nil, t.Ptr().Ast()),
				Field(nil, ErrorType.Ast()),
			),
		},
		Body: &ast.BlockStmt{List: statements},
	}

	g.addStatement(n)

	return nil
}

func (g *Generator) generateFieldUnmarshaler(f *FieldDescriptor) (ast.Stmt, error) {
	expr, err := g.generateFieldUnmarshalExpression(f.Type)
	if err != nil {
		return nil, err
	}

	var varErr = Ident("err")
	return &ast.CaseClause{
		List: []ast.Expr{String(f.Name)},
		Body: []ast.Stmt{
			MultiAssign([]ast.Expr{
				Select(ast.NewIdent("result"), ast.NewIdent(g.title(f.Name))),
				varErr,
			}, []ast.Expr{expr}),
		},
	}, nil
}

func (g Generator) generateFieldUnmarshaler_string() (ast.Expr, error) {
	return Call(Select(ast.NewIdent("r"), ast.NewIdent("NextOptionalString"))), nil
}

func (g *Generator) generateStructField(o *StructDescriptor, f *FieldDescriptor) *ast.Field {
	t := g.generateType(f.Type)

	if t.Kind != TypeSlice && t.Kind != TypeInterface {
		t = t.Ptr()
	}

	return &ast.Field{
		Names: []*ast.Ident{Ident(g.title(f.Name))},
		Type:  t.Ast(),
		Doc:   docComment(g.title(f.Name), f.Description),
	}
}

func (g *Generator) registerType(kind TypeKind, name string) Type {
	t := Type{Kind: kind, Name: name}
	g.typeMap[name] = Type{Kind: kind, Name: name}
	return t
}

func (g *Generator) lookupType(s string) Type {
	t, ok := g.typeMap[s]
	if !ok {
		t = Type{Kind: TypeStruct, Name: s}
		g.typeMap[s] = t
	}

	return t
}

func (g *Generator) addStatement(n ast.Decl) {
	g.nodes = append(g.nodes, n)
}

func (g *Generator) generateStructFieldsOn(each *StructDescriptor) ([]*ast.Field, error) {
	fields, err := g.collectStructFields(each)
	if err != nil {
		return nil, err
	}

	var sfields []*ast.Field

	for _, f := range fields {
		field := g.generateStructField(f.type_, f.field)
		sfields = append(sfields, field)
	}

	return sfields, nil
}

func (g *Generator) collectStructFieldsIn(o *StructDescriptor, fields *fieldTypeCollection) error {
	if len(o.Extends) > 0 {
		t := g.document.structByName(o.Extends)
		if t == nil {
			return fmt.Errorf("super type not found")
		}
		err := g.collectStructFieldsIn(t, fields)
		if err != nil {
			return err
		}
	}

	for _, field := range o.Fields {
		fields.add(fieldTypePair{
			field: field,
			type_: o,
		})
	}

	return nil
}

func (g *Generator) generateOneOfType(o *StructDescriptor, f *FieldDescriptor) Type {
	tn := "OneOf" + o.Name + g.title(f.Name)

	var fields []*ast.Field

	for _, each := range f.OneOf {
		n := g.lookupType(each)
		fields = append(fields, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(g.title(each))},
			Type:  n.Ptr().Ast(),
		})
	}

	s := Struct(Ident(tn), fields)

	g.addStatement(s)
	return g.registerType(TypeStruct, tn)
}

func (g Generator) title(name string) string {
	return strings.ToUpper(name[0:1]) + name[1:]
}

func (g *Generator) generateInterfaceType(each *StructDescriptor) error {
	var fields []*ast.Field

	if len(each.Extends) > 0 {
		parent := g.lookupType(each.Extends)

		fields = append(fields, &ast.Field{
			Type: parent.Ast(),
		})
	}

	i := &ast.GenDecl{
		Doc: docComment(each.Name, each.Description),
		Tok: token.TYPE,
		Specs: []ast.Spec{&ast.TypeSpec{
			Name: ast.NewIdent(each.Name),
			Type: &ast.InterfaceType{
				Methods: FieldList(fields...),
			},
		}},
	}

	g.addStatement(i)
	g.registerType(TypeInterface, each.Name)
	return nil
}

func docComment(subject, description string) *ast.CommentGroup {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}
	paragraphs := strings.Split(description, "\n")
	var comments []*ast.Comment
	for paragraphIndex, paragraph := range paragraphs {
		if paragraphIndex > 0 {
			comments = append(comments, &ast.Comment{Text: "//"})
		}
		prefix := ""
		if paragraphIndex == 0 {
			prefix = subject + ": "
		}
		for _, line := range wrapComment(prefix+paragraph, 100) {
			comments = append(comments, &ast.Comment{Text: "// " + line})
		}
	}
	return &ast.CommentGroup{List: comments}
}

func wrapComment(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if len(lines[last])+1+len(word) <= width {
			lines[last] += " " + word
		} else {
			lines = append(lines, word)
		}
	}
	return lines
}

func (g Generator) collectStructFields(each *StructDescriptor) ([]fieldTypePair, error) {
	fields := &fieldTypeCollection{
		order: map[string]int{},
	}

	err := g.collectStructFieldsIn(each, fields)
	if err != nil {
		return nil, err
	}
	return fields.fields, nil
}

func (g Generator) generateFieldUnmarshaler_interface(f *FieldDescriptor) (ast.Stmt, error) {
	return Return(Nil, Nil), nil
}

func (g Generator) generateFieldUnmarshaler_struct(t Type) (ast.Expr, error) {
	return Call(Ident("unmarshal"+t.Name), ast.NewIdent("r")), nil
}

func (g Generator) generateFieldUnmarshalExpression(t *TypeDescriptor) (ast.Expr, error) {
	var expr ast.Expr
	var err error

	if t == nil {
		expr, err = g.generateFieldUnmarshaler_string()
		if err != nil {
			return nil, err
		}
	} else if t.IsUnion() {

	} else if t.IsList() {
		elem, err := g.generateFieldUnmarshalExpression(t.Elems[0])
		if err != nil {
			return nil, err
		}

		s := g.document.structByName(t.Elems[0].Name)
		isAbstract := s != nil && s.Abstract

		f := ast.NewIdent("unmarshalList")
		elemType := g.generateType(t.Elems[0]).Ast()

		if isAbstract {
			f = ast.NewIdent("unmarshalAbstractList")
		} else {
			elemType = Ptr(elemType)
		}

		expr = Call(
			f,
			ast.NewIdent("r"),
			&ast.FuncLit{
				Type: &ast.FuncType{
					Params: FieldList(
						Field(Ident("r"), Ptr(Ident("JSONReader")))),
					Results: FieldList(
						Field(nil, elemType),
						Field(nil, Ident("error"))),
				},
				Body: Block(
					Return(elem),
				),
			},
		)
	} else {
		t := g.lookupType(t.Name)

		if t.Kind == TypeBuiltin {
			switch t.Name {
			case "string":
				expr, err = g.generateFieldUnmarshaler_string()
				if err != nil {
					return nil, err
				}
			case "bool":
				expr = Call(Select(ast.NewIdent("r"), ast.NewIdent("NextOptionalBool")))

			case "int":
				expr = Call(Select(ast.NewIdent("r"), ast.NewIdent("NextOptionalInt")))

			case "float64":
				expr = Call(Select(ast.NewIdent("r"), ast.NewIdent("NextOptionalFloat64")))

			case "any":
				expr = Call(Select(ast.NewIdent("r"), ast.NewIdent("NextOptionalValue")))

			default:
				panic(fmt.Sprintf("unknown type %s", t.Name))
			}
		} else if t.Kind == TypeStruct {
			expr, err = g.generateFieldUnmarshaler_struct(t)
			if err != nil {
				return nil, err
			}
		} else if t.Kind == TypeInterface {
			expr, err = g.generateFieldUnmarshaler_struct(t)
			if err != nil {
				return nil, err
			}
		}
	}

	return expr, nil
}

func (g Generator) generateType(d *TypeDescriptor) Type {
	if d == nil {
		return g.lookupType("string")
	} else if d.IsUnion() {
		panic("Not implemented yet")
	} else if d.IsList() {
		return g.generateType(d.Elems[0]).Slice()
	} else {
		return g.lookupType(d.Name)
	}
}

func (g *Generator) generateInterfaceUnmarshaller(s *StructDescriptor, children []StructDescriptor) error {
	varT := Ident("t")
	varErr := Ident("err")
	varReader := Ident("r")

	var cases []ast.Stmt

	for _, each := range children {
		cases = append(cases, &ast.CaseClause{
			List: Exprs(String(each.Name)),
			Body: Stmts(
				Return(Call(Ident("unmarshal"+each.Name), varReader)),
			),
		})
	}

	cases = append(cases, &ast.CaseClause{
		Body: Stmts(
			Return(Nil, formatError("unknown type %s", varT)),
		),
	})

	body := Block(
		MultiDefine([]ast.Expr{varT, varErr}, []ast.Expr{Call(
			Ident("determineTypeDiscriminator"),
			varReader,
			String("$type"),
		)}),
		If(NotNil(varErr), Block(Return(Nil, varErr))),
		&ast.SwitchStmt{
			Tag: varT,
			Body: Block(
				cases...,
			),
		},
	)

	i := &ast.FuncDecl{
		Name: ast.NewIdent("unmarshal" + s.Name),
		Type: &ast.FuncType{
			Params: FieldList(
				Field(Ident("r"), Ptr(Ident("JSONReader"))),
			),
			Results: FieldList(
				Field(nil, Ident(s.Name)),
				Field(nil, Ident("error")),
			),
		},
		Body: body,
	}

	g.addStatement(i)
	return nil
}

func formatError(s string, args ...ast.Expr) ast.Expr {
	args = append(
		[]ast.Expr{String(s)},
		args...,
	)

	return Call(
		Select(Ident("fmt"), Ident("Errorf")),
		args...,
	)
}

func (g *Generator) generateInterfaceMarshaller(s *StructDescriptor, subtypes []StructDescriptor) error {
	valVar := Ident("v")
	vtVar := Ident("vt")
	writerVar := Ident("w")

	cases := []ast.Stmt{}
	for _, each := range subtypes {
		if each.Abstract {
			continue
		}

		cases = append(cases, &ast.CaseClause{
			List: Exprs(Ident(each.Name)),
			Body: Stmts(Return(Call(Ident("marshal"+each.Name), writerVar, vtVar))),
		})
	}
	cases = append(cases, &ast.CaseClause{
		Body: Stmts(
			Return(formatError("unknown type %T", valVar)),
		),
	})

	stmts := []ast.Stmt{
		&ast.TypeSwitchStmt{
			Assign: Define(vtVar, &ast.TypeAssertExpr{X: valVar}),
			Body:   Block(cases...),
		},
	}

	f := &ast.FuncDecl{
		Name: Ident("marshal" + s.Name),
		Type: &ast.FuncType{Params: FieldList(
			Field(writerVar, Ptr(Ident("JsonMarshaler"))),
			Field(valVar, Ident(s.Name)),
		),
			Results: FieldList(Field(nil, Ident("error")))},
		Body: Block(
			stmts...,
		),
	}

	g.addStatement(f)
	return nil
}

func (g *Generator) generateStructMarshaler(s *StructDescriptor) error {
	valVar := Ident("v")
	writerVar := Ident("w")
	errVar := Ident("err")

	guard := func(x ast.Expr) ast.Stmt {
		return &ast.IfStmt{
			Init: Assign(errVar, x),
			Cond: NotNil(errVar),
			Body: Block(Return(errVar)),
		}
	}

	stmts := []ast.Stmt{
		Declare(Ident("err"), Ident("error")),
		guard(Call(Select(writerVar, Ident("ObjectStart")))),

		&ast.ExprStmt{X: Call(Select(writerVar, Ident("WriteKey")), String("$type"))},
		&ast.ExprStmt{X: Call(Select(writerVar, Ident("WriteString")), String(s.Name))},
	}

	fields, e := g.collectStructFields(s)
	if e != nil {
		return e
	}

	for _, each := range fields {
		e := g.generateFieldMarshalExpr(each.field, valVar, writerVar)

		stmts = append(stmts, If(
			NotNil(Select(valVar, Ident(g.title(each.field.Name)))),
			Block(
				guard(Call(
					Select(writerVar, Ident("WriteKey")),
					String(each.field.Name),
				)),
				guard(e),
			),
		))
	}

	stmts = append(stmts,
		guard(Call(Select(writerVar, Ident("ObjectEnd")))),
		Return(Nil))

	f := &ast.FuncDecl{
		Name: Ident("marshal" + s.Name),
		Type: &ast.FuncType{Params: FieldList(
			Field(writerVar, Ptr(Ident("JsonMarshaler"))),
			Field(valVar, Ident(s.Name)),
		),
			Results: FieldList(Field(nil, Ident("error")))},
		Body: Block(
			stmts...,
		),
	}

	g.addStatement(f)
	return nil
}

func (g Generator) generateFieldMarshalExpr(field *FieldDescriptor, valVar *ast.Ident, writerVar *ast.Ident) ast.Expr {
	t := g.generateType(field.Type)
	switch t.Kind {

	case TypeBuiltin:
		f := Ptr(Select(valVar, Ident(g.title(field.Name))))

		switch t.Name {
		case "string":
			return Call(Select(writerVar, Ident("WriteString")), f)
		case "int":
			return Call(
				Select(writerVar, Ident("WriteInt")),
				f,
			)
		case "bool":
			return Call(
				Select(writerVar, Ident("WriteBool")),
				f,
			)
		case "float64":
			return Call(
				Select(writerVar, Ident("WriteFloat64")),
				f,
			)
		case "any":
			return Call(
				Select(writerVar, Ident("WriteValue")),
				f,
			)
		}

	case TypeStruct:
		return Call(
			Ident("marshal"+t.Name),
			writerVar,
			Ptr(Select(valVar, Ident(g.title(field.Name)))),
		)

	case TypeInterface:
		return Call(
			Ident("marshal"+t.Name),
			writerVar,
			Select(valVar, Ident(g.title(field.Name))),
		)

	case TypeSlice:
		return Call(
			Ident("marshalList"),
			writerVar,
			Select(valVar, Ident(g.title(field.Name))),
			Ident("marshal"+t.Elem.Name),
		)

	default:
		panic("unhandled default case")
	}

	panic("unhandled default case")

}
