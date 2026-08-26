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
