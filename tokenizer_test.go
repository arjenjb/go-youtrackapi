package youtrackapi

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
