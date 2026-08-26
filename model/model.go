package model

import "github.com/arjenjb/go-youtrackapi/internal/util"

type NamedList[T any] []T

type Document struct {
	Structs NamedList[*StructDescriptor]
}

func (d Document) structByName(name string) *StructDescriptor {
	for _, each := range d.Structs {
		if each.Name == name {
			return each
		}
	}
	return nil
}

func (d Document) AllParentsOf(each StructDescriptor) []StructDescriptor {
	if len(each.Extends) == 0 {
		return nil
	}

	var result []StructDescriptor
	t := d.structByName(each.Extends)
	if t != nil {
		result = append(result, *t)
		result = append(result, d.AllParentsOf(*t)...)
	}

	return result
}

func Contains[T comparable](l util.LinkedList[T], el T) bool {
	for each := l.Head; each != nil; each = each.Next {
		if each.E == el {
			return true
		}
	}
	return false
}

func (d Document) AllChildrenOf(parent StructDescriptor) []StructDescriptor {
	names := map[string]bool{}
	var children []StructDescriptor

	todo := util.LinkedList[string]{}
	todo.Append(parent.Name)

	for !todo.IsEmpty() {
		p := todo.RemoveFirst()

		for _, each := range d.Structs {
			if each.Extends == p {
				if _, ok := names[each.Name]; !ok {
					todo.Append(each.Name)
					names[each.Name] = true
					children = append(children, *each)
				}
			}
		}
	}

	return children
}

func (d Document) ChildrenOf(parent StructDescriptor) []StructDescriptor {
	var children []StructDescriptor
	for _, each := range d.Structs {
		if each.Extends == parent.Name {
			children = append(children, *each)
		}
	}
	return children
}

type StructDescriptor struct {
	Name        string
	Description string
	Extends     string
	Abstract    bool
	Fields      NamedList[*FieldDescriptor]
}

type TypeDescriptorKind uint8

const (
	TypeDescriptorKindUnknown TypeDescriptorKind = iota
	TypeDescriptorKindBasic
	TypeDescriptorKindList
	TypeDescriptorKindUnion
)

type TypeDescriptor struct {
	Kind  TypeDescriptorKind
	Name  string
	Elems []*TypeDescriptor
}

func (t *TypeDescriptor) IsUnion() bool {
	return t.Kind == TypeDescriptorKindUnion
}

func (t *TypeDescriptor) IsList() bool {
	return t.Kind == TypeDescriptorKindList
}

type FieldDescriptor struct {
	Name        string
	Description string
	Type        *TypeDescriptor
	OneOf       []string
	Enum        []string
}
