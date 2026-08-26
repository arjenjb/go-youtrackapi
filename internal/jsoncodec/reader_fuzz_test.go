package jsoncodec

import (
	"bytes"
	"testing"
)

func FuzzReaderDoesNotPanic(f *testing.F) {
	seeds := [][]byte{
		{},
		[]byte(" "),
		[]byte(`"`),
		[]byte(`"\`),
		[]byte(`"\x"`),
		[]byte(`"\u12"`),
		[]byte(`"\uD83D\uDE00"`),
		[]byte("-"),
		[]byte("1."),
		[]byte("1e+"),
		[]byte("[true, null,"),
		[]byte(`{"key":[1,2,3]}`),
		{0xff, 0xfe, 0xfd},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		reader := NewReader(bytes.NewReader(input))
		for range len(input) + 2 {
			token, err := reader.Next()
			if err != nil || token.IsEOF() {
				return
			}
		}

		t.Fatal("reader did not reach EOF or return an error")
	})
}
