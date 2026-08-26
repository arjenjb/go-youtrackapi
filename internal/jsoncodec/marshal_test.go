package jsoncodec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshallerString(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	err := m.WriteString("x")
	require.NoError(t, err)

	require.Equal(t, `"x"`, buff.String())
}

func TestMarshallerBool(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	err := m.WriteBool(true)
	require.NoError(t, err)

	require.Equal(t, "true", buff.String())
}

func TestMarshallerArrayEmpty(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	err := m.ArrayStart()
	require.NoError(t, err)
	err = m.ArrayEnd()
	require.NoError(t, err)

	require.Equal(t, "[]", buff.String())
}

func TestMarshallerArraySingleElement(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	err := m.ArrayStart()
	require.NoError(t, err)
	err = m.WriteBool(true)
	require.NoError(t, err)
	err = m.ArrayEnd()
	require.NoError(t, err)

	require.Equal(t, "[true]", buff.String())
}

func TestMarshallerArrayTwoElements(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	err := m.ArrayStart()
	require.NoError(t, err)
	err = m.WriteBool(true)
	require.NoError(t, err)
	err = m.WriteBool(false)
	require.NoError(t, err)
	err = m.ArrayEnd()
	require.NoError(t, err)

	require.Equal(t, "[true,false]", buff.String())
}

func TestMarshallerNestedArray(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	m.ArrayStart()
	m.ArrayStart()
	m.ArrayStart()
	m.WriteBool(true)
	m.ArrayEnd()
	m.ArrayEnd()
	m.ArrayEnd()

	require.Equal(t, "[[[true]]]", buff.String())
}

func TestMarshallerEmptyObject(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	m.ObjectStart()
	m.ObjectEnd()

	require.Equal(t, "{}", buff.String())
}

func TestMarshallerObjectSingleKey(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	m.ObjectStart()
	m.WriteKey("a")
	m.WriteBool(true)
	m.ObjectEnd()

	require.Equal(t, `{"a":true}`, buff.String())
}

func TestMarshallerObjectMultipleKeys(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	m.ObjectStart()
	m.WriteKey("a")
	m.WriteBool(true)
	m.WriteKey("b")
	m.WriteInt(42)
	m.WriteKey("c")
	m.WriteNull()
	m.WriteKey("d")
	m.ArrayStart()
	m.ArrayEnd()
	m.WriteKey("e")
	m.ArrayStart()
	m.WriteBool(false)
	m.ArrayEnd()
	m.ObjectEnd()

	require.Equal(t, `{"a":true,"b":42,"c":null,"d":[],"e":[false]}`, buff.String())
}

func TestMarshallerArrayOfObjects(t *testing.T) {
	buff := strings.Builder{}
	m := NewMarshaler(&buff)
	m.ArrayStart()
	m.ObjectStart()
	m.ObjectEnd()
	m.ObjectStart()
	m.ObjectEnd()
	m.ArrayEnd()

	require.Equal(t, "[{},{}]", buff.String())
}
