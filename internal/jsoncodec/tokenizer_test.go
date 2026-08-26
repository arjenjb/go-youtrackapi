package jsoncodec

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenizer_Unicode(t *testing.T) {
	tsts := []struct {
		name   string
		input  string
		expect string
	}{{
		name:   "a",
		input:  `"\u0061"`,
		expect: "a",
	}, {
		name:   "special char",
		input:  `"\u23FB"`,
		expect: "⏻",
	}, {
		name:   "mixedcase",
		input:  `"\uaAaA"`,
		expect: "ꪪ",
	}, {
		name:   "lowercase",
		input:  `"\uaAaA"`,
		expect: "ꪪ",
	}, {
		name:   "uppercase",
		input:  `"\uAAAA"`,
		expect: "ꪪ",
	}, {
		name:   "trailing",
		input:  `"\u0061a"`,
		expect: "aa",
	}}

	for _, each := range tsts {
		t.Run(each.name, func(t *testing.T) {
			s := StreamingJsonTokenizer{
				r:     bufio.NewReader(strings.NewReader(each.input)),
				atEnd: false,
			}

			tok, err := s.Next()
			require.NoError(t, err)
			require.Equal(t, each.expect, tok.StringValue)
		})
	}

}

func TestTokenizer_UnicodeSurrogatePair(t *testing.T) {
	token, err := newStreamingTokenizer(`"\uD83D\uDE00"`).Next()
	require.NoError(t, err)
	require.Equal(t, String, token.Type)
	require.Equal(t, "😀", token.StringValue)
}

func TestTokenizer_TrailingWhitespaceReturnsEOF(t *testing.T) {
	tests := []string{" ", "\n", "\t\r ", "true \n\t"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tokenizer := newStreamingTokenizer(input)
			if strings.HasPrefix(input, "true") {
				token, err := tokenizer.Next()
				require.NoError(t, err)
				require.Equal(t, Bool, token.Type)
			}

			token, err := tokenizer.Next()
			require.NoError(t, err)
			require.Equal(t, EOF, token.Type)
		})
	}
}

func TestTokenizer_ExponentNumbers(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{input: "1e3", want: 1000},
		{input: "1E+2", want: 100},
		{input: "-2e-2", want: -0.02},
		{input: "6.02e23", want: 6.02e23},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			token, err := newStreamingTokenizer(tt.input).Next()
			require.NoError(t, err)
			require.Equal(t, Float, token.Type)
			require.Equal(t, tt.want, token.FloatValue)
		})
	}
}

func TestTokenizer_RejectsInvalidNumbers(t *testing.T) {
	inputs := []string{
		"-",
		"1.",
		"1e",
		"1e+",
		"01",
		"-01",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := newStreamingTokenizer(input).Next()
			require.Error(t, err)
		})
	}
}

func TestTokenizer_RejectsTruncatedStringsAndInvalidEscapes(t *testing.T) {
	inputs := []string{
		`"`,
		`"value`,
		`"\`,
		`"\x"`,
		`"\u12"`,
		`"\u12xz"`,
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := newStreamingTokenizer(input).Next()
			require.Error(t, err)
		})
	}
}

func newStreamingTokenizer(input string) *StreamingJsonTokenizer {
	return &StreamingJsonTokenizer{r: bufio.NewReader(strings.NewReader(input))}
}
