package jsoncodec

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDetermineTypeDiscriminatorFindsValueAndRewinds(t *testing.T) {
	reader := NewReader(strings.NewReader(`{"id":"1","nested":{"value":2},"$type":"Issue"}`))

	typeName, err := DetermineTypeDiscriminator(reader, "$type")
	require.NoError(t, err)
	require.Equal(t, "Issue", typeName)

	value, err := reader.NextValue()
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"id":     "1",
		"nested": map[string]any{"value": 2},
		"$type":  "Issue",
	}, value)
}

func TestDetermineTypeDiscriminatorRejectsInvalidObjects(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "missing discriminator", input: `{"id":"1"}`},
		{name: "non-string discriminator", input: `{"$type":42}`},
		{name: "not an object", input: `[]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := NewReader(strings.NewReader(test.input))
			_, err := DetermineTypeDiscriminator(reader, "$type")
			require.Error(t, err)
		})
	}
}

func TestMarshalScalarHelpers(t *testing.T) {
	timestamp := time.Date(2026, time.August, 26, 12, 34, 56, 789_000_000, time.FixedZone("test", 2*60*60))
	tests := []struct {
		name    string
		want    string
		marshal func(*Marshaler) error
	}{
		{name: "string", want: `"value"`, marshal: func(w *Marshaler) error { return MarshalString(w, "value") }},
		{name: "bool", want: "true", marshal: func(w *Marshaler) error { return MarshalBool(w, true) }},
		{name: "int", want: "42", marshal: func(w *Marshaler) error { return MarshalInt(w, 42) }},
		{name: "float", want: "3.5", marshal: func(w *Marshaler) error { return MarshalFloat64(w, 3.5) }},
		{name: "any", want: `{"ok":true}`, marshal: func(w *Marshaler) error {
			return MarshalAny(w, map[string]bool{"ok": true})
		}},
		{name: "time", want: "1787740496789", marshal: func(w *Marshaler) error { return MarshalTime(w, timestamp) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output strings.Builder
			err := test.marshal(NewMarshaler(&output))
			require.NoError(t, err)
			require.Equal(t, test.want, output.String())
		})
	}
}

func TestMarshalList(t *testing.T) {
	var output strings.Builder
	err := MarshalList(NewMarshaler(&output), []string{"one", "two"}, MarshalString)

	require.NoError(t, err)
	require.Equal(t, `["one","two"]`, output.String())
}

func TestMarshalListPropagatesElementError(t *testing.T) {
	wantErr := errors.New("element failed")
	err := MarshalList(NewMarshaler(&strings.Builder{}), []int{1}, func(*Marshaler, int) error {
		return wantErr
	})

	require.ErrorIs(t, err, wantErr)
}

func TestUnmarshalTime(t *testing.T) {
	t.Run("milliseconds", func(t *testing.T) {
		got, err := UnmarshalTime(NewReader(strings.NewReader("1787740496789")))

		require.NoError(t, err)
		require.Equal(t, time.Date(2026, time.August, 26, 10, 34, 56, 789_000_000, time.UTC), *got)
	})

	t.Run("null", func(t *testing.T) {
		got, err := UnmarshalTime(NewReader(strings.NewReader("null")))

		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := UnmarshalTime(NewReader(strings.NewReader(`"invalid"`)))

		require.Error(t, err)
		require.Nil(t, got)
	})
}

func TestUnmarshalLists(t *testing.T) {
	t.Run("concrete", func(t *testing.T) {
		got, err := UnmarshalList(NewReader(strings.NewReader("[1,2,3]")), func(reader *Reader) (*int, error) {
			return reader.NextOptionalInt()
		})

		require.NoError(t, err)
		require.Equal(t, []int{1, 2, 3}, got)
	})

	t.Run("abstract", func(t *testing.T) {
		got, err := UnmarshalAbstractList(NewReader(strings.NewReader(`["one","two"]`)), func(reader *Reader) (any, error) {
			value, err := reader.NextOptionalString()
			if err != nil {
				return nil, err
			}
			return *value, nil
		})

		require.NoError(t, err)
		require.Equal(t, []any{"one", "two"}, got)
	})
}

func TestUnmarshalListPropagatesElementError(t *testing.T) {
	wantErr := errors.New("element failed")
	_, err := UnmarshalList(NewReader(strings.NewReader("[1]")), func(*Reader) (*int, error) {
		return nil, wantErr
	})

	require.ErrorIs(t, err, wantErr)
}
