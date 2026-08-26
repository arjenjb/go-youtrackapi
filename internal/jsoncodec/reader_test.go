package jsoncodec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJsonReader_Buffering(t *testing.T) {
	input := `1 2 3 4 5`
	j := NewReader(strings.NewReader(input))

	requireNextToken(t, j, Token{Type: Int, IntValue: 1})
	requireNextToken(t, j, Token{Type: Int, IntValue: 2})

	j.Buffer()
	requireNextToken(t, j, Token{Type: Int, IntValue: 3})
	requireNextToken(t, j, Token{Type: Int, IntValue: 4})
	j.Rewind()

	requireNextToken(t, j, Token{Type: Int, IntValue: 3})
	requireNextToken(t, j, Token{Type: Int, IntValue: 4})
	requireNextToken(t, j, Token{Type: Int, IntValue: 5})
	requireNextToken(t, j, Token{Type: EOF})
}

func requireNextToken(t *testing.T, j *Reader, expected Token) {
	got, err := j.Next()
	require.NoError(t, err)
	require.Equal(t, expected, *got)
}

func TestJsonReader_True(t *testing.T) {
	j := NewReader(strings.NewReader("true"))
	tok, err := j.Next()
	require.NoError(t, err)
	require.Equal(t, Bool, tok.Type)
	requireNextEOF(t, j)
}

func requireNextEOF(t *testing.T, j *Reader) {
	requireNextType(t, j, EOF)
}

func requireNextType(t *testing.T, j *Reader, tp TokenType) {
	tok, err := j.Next()
	require.NoError(t, err)
	require.Equal(t, tp, tok.Type)
}

func TestJsonReader_False(t *testing.T) {
	j := NewReader(strings.NewReader("false"))
	requireNextType(t, j, Bool)
	requireNextType(t, j, EOF)
}

func TestJsonReader_List(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenType
	}{
		{
			`""`,
			[]TokenType{String},
		},
		{
			`"a"`,
			[]TokenType{String},
		},
		{
			`"b"`,
			[]TokenType{String},
		},
		{
			`"\""`,
			[]TokenType{String},
		},
		{
			"42",
			[]TokenType{Int},
		},
		{
			"-1",
			[]TokenType{Int},
		},
		{
			"1.0",
			[]TokenType{Float},
		},
		{
			"3.1415926",
			[]TokenType{Float},
		},
		{
			"[ true ]",
			[]TokenType{ArrayBegin, Bool, ArrayClose},
		},
		{
			"[ true, false ]",
			[]TokenType{ArrayBegin, Bool, Comma, Bool, ArrayClose},
		},
		{
			"[ false, null ]",
			[]TokenType{ArrayBegin, Bool, Comma, Null, ArrayClose},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {

			j := NewReader(strings.NewReader(tt.input))
			got := []TokenType{}

			for {
				tok, err := j.Next()
				require.NoError(t, err)
				if tok.IsEOF() {
					break
				}

				got = append(got, tok.Type)
			}

			require.Equal(t, got, tt.expected)
		})
	}
}

func TestReader_NextOptionalBool(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *bool
		wantErr bool
	}{
		{name: "true", input: "true", want: boolPointer(true)},
		{name: "false", input: "false", want: boolPointer(false)},
		{name: "null", input: "null"},
		{name: "wrong type", input: `"true"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReader(strings.NewReader(tt.input)).NextOptionalBool()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestReader_NextOptionalFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *float64
		wantErr bool
	}{
		{name: "float", input: "1.5", want: float64Pointer(1.5)},
		{name: "integer", input: "2", want: float64Pointer(2)},
		{name: "null", input: "null"},
		{name: "wrong type", input: "false", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReader(strings.NewReader(tt.input)).NextOptionalFloat64()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestReader_NextOptionalValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    any
		wantNil bool
	}{
		{name: "boolean", input: "true", want: true},
		{name: "string", input: `"value"`, want: "value"},
		{name: "integer", input: "42", want: 42},
		{name: "float", input: "1.25", want: 1.25},
		{name: "list", input: `[1,"two"]`, want: []any{1, "two"}},
		{name: "object", input: `{"key":false}`, want: map[string]any{"key": false}},
		{name: "null", input: "null", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReader(strings.NewReader(tt.input)).NextOptionalValue()
			require.NoError(t, err)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			require.Equal(t, tt.want, *got)
		})
	}
}

func TestReader_NextList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []any
	}{
		{name: "empty", input: "[]", want: []any{}},
		{
			name:  "mixed and nested",
			input: `[true,null,"value",2,1.5,[false],{"key":"value"}]`,
			want: []any{
				true,
				nil,
				"value",
				2,
				1.5,
				[]any{false},
				map[string]any{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReader(strings.NewReader(tt.input)).NextList()
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func float64Pointer(value float64) *float64 {
	return &value
}
