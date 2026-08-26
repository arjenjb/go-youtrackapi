package jsoncodec_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/arjenjb/go-youtrackapi/internal/jsoncodec"
	"github.com/stretchr/testify/require"
)

var errWriteFailed = errors.New("write failed")

type failWriter struct {
	failAt int
	writes int
}

func (w *failWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, errWriteFailed
	}
	return len(p), nil
}

func TestMarshalerRejectsObjectKeyOutsideObject(t *testing.T) {
	m := jsoncodec.NewMarshaler(&strings.Builder{})

	err := m.WriteKey("key")

	require.EqualError(t, err, "unexpected object key")
}

func TestMarshalerRejectsObjectEndOutsideObject(t *testing.T) {
	m := jsoncodec.NewMarshaler(&strings.Builder{})

	err := m.ObjectEnd()

	require.EqualError(t, err, "unexpected object end token")
}

func TestMarshalerRejectsObjectEndWithoutValue(t *testing.T) {
	m := jsoncodec.NewMarshaler(&strings.Builder{})
	require.NoError(t, m.ObjectStart())
	require.NoError(t, m.WriteKey("key"))

	err := m.ObjectEnd()

	require.EqualError(t, err, "unexpected object end token")
}

func TestMarshalerRejectsMismatchedEndTokens(t *testing.T) {
	t.Run("object end in array", func(t *testing.T) {
		m := jsoncodec.NewMarshaler(&strings.Builder{})
		require.NoError(t, m.ArrayStart())

		err := m.ObjectEnd()

		require.EqualError(t, err, "unexpected object end token")
	})

	t.Run("array end in object", func(t *testing.T) {
		m := jsoncodec.NewMarshaler(&strings.Builder{})
		require.NoError(t, m.ObjectStart())

		err := m.ArrayEnd()

		require.EqualError(t, err, "unexpected array end token")
	})
}

func TestMarshalerPropagatesWriterErrors(t *testing.T) {
	tests := map[string]func(io.Writer) error{
		"string": func(w io.Writer) error {
			return jsoncodec.NewMarshaler(w).WriteString("value")
		},
		"scalar": func(w io.Writer) error {
			return jsoncodec.NewMarshaler(w).WriteInt(42)
		},
		"object start delimiter": func(w io.Writer) error {
			return jsoncodec.NewMarshaler(w).ObjectStart()
		},
		"array start delimiter": func(w io.Writer) error {
			return jsoncodec.NewMarshaler(w).ArrayStart()
		},
	}

	for name, marshal := range tests {
		t.Run(name, func(t *testing.T) {
			err := marshal(&failWriter{failAt: 1})

			require.ErrorIs(t, err, errWriteFailed)
		})
	}
}

func TestMarshalerPropagatesClosingDelimiterWriterErrors(t *testing.T) {
	tests := map[string]struct {
		start func(*jsoncodec.Marshaler) error
		end   func(*jsoncodec.Marshaler) error
	}{
		"object": {(*jsoncodec.Marshaler).ObjectStart, (*jsoncodec.Marshaler).ObjectEnd},
		"array":  {(*jsoncodec.Marshaler).ArrayStart, (*jsoncodec.Marshaler).ArrayEnd},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			writer := &failWriter{failAt: 2}
			m := jsoncodec.NewMarshaler(writer)
			require.NoError(t, test.start(m))

			err := test.end(m)

			require.ErrorIs(t, err, errWriteFailed)
		})
	}
}

func TestMarshalListPropagatesWriterError(t *testing.T) {
	writer := &failWriter{failAt: 2}
	m := jsoncodec.NewMarshaler(writer)

	err := jsoncodec.MarshalList(m, []string{"value"}, jsoncodec.MarshalString)

	require.ErrorIs(t, err, errWriteFailed)
}
