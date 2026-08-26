package youtrackapi

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type TokenType int

const (
	ArrayBegin TokenType = iota
	ArrayClose
	ObjectBegin
	ObjectClose
	Colon
	Comma
	String
	Int
	Float
	Bool
	Null
	EOF
)

type Token struct {
	Type        TokenType
	StringValue string
	FloatValue  float64
	IntValue    int
	BoolValue   bool
}

func (t Token) IsEOF() bool {
	return t.Type == EOF
}

func (t Token) String() string {
	switch t.Type {
	case ArrayBegin:
		return "ArrayBegin"
	case ArrayClose:
		return "ArrayClose"
	case ObjectBegin:
		return "ObjectBegin"
	case ObjectClose:
		return "ObjectClose"
	case Colon:
		return "Colon"
	case Comma:
		return "Comma"
	case String:
		return "String"
	case Int:
		return "Int"
	case Float:
		return "Float"
	case Bool:
		return "Bool"
	case Null:
		return "Null"
	case EOF:
		return "EOF"
	default:
		return "<unknown>"
	}
}

func (t Token) IsNull() bool {
	return t.Type == Null
}

type StreamingJsonTokenizer struct {
	r     *bufio.Reader
	atEnd bool
}

func (j *StreamingJsonTokenizer) AtEnd() bool {
	_, err := j.r.Peek(1)
	return err == io.EOF
}

func (j *StreamingJsonTokenizer) Next() (*Token, error) {
	peek, err := j.r.Peek(1)
	if err == io.EOF {
		return j.newToken(EOF), nil
	} else if err != nil {
		return nil, err
	}
	c := rune(peek[0])

	// Skip whitespace
	for isWhitespace(c) {
		_, err = j.r.Discard(1)
		if err != nil {
			return nil, err
		}

		peek, err = j.r.Peek(1)
		c = rune(peek[0])
		if err != nil {
			return nil, err
		}
	}

	if c >= '0' && c <= '9' {
		return j.nextNumber()
	}

	switch rune(c) {
	case '[':
		_, err = j.r.Discard(1)
		return &Token{Type: ArrayBegin}, err
	case ']':
		_, err = j.r.Discard(1)
		return &Token{Type: ArrayClose}, err
	case '{':
		_, err = j.r.Discard(1)
		return &Token{Type: ObjectBegin}, err
	case '}':
		_, err = j.r.Discard(1)
		return &Token{Type: ObjectClose}, err
	case ',':
		_, err = j.r.Discard(1)
		return &Token{Type: Comma}, err
	case ':':
		_, err = j.r.Discard(1)
		return &Token{Type: Colon}, err
	case '"':
		return j.nextString()
	case 'f':
		val, err := j.r.Peek(5)
		if err != nil {
			return nil, err
		}
		if string(val) != "false" {
			return nil, fmt.Errorf("expected false")
		}
		j.r.Discard(5)
		return &Token{Type: Bool, BoolValue: false}, err
	case 't':
		val, err := j.r.Peek(4)
		if err != nil {
			return nil, err
		}
		if string(val) != "true" {
			return nil, fmt.Errorf("expected true")
		}
		j.r.Discard(4)
		return &Token{Type: Bool, BoolValue: true}, err
	case 'n':
		val, err := j.r.Peek(4)
		if err != nil {
			return nil, err
		}
		if string(val) != "null" {
			return nil, fmt.Errorf("expected null")
		}
		j.r.Discard(4)
		return &Token{Type: Null}, err

	case '-':
		return j.nextNumber()

	default:
		return nil, fmt.Errorf("unexpected character '%c'", c)
	}
}

func (j *StreamingJsonTokenizer) newToken(t TokenType) *Token {
	return &Token{
		Type: t,
	}
}

func (j *StreamingJsonTokenizer) nextString() (*Token, error) {
	buf := strings.Builder{}
	j.r.Discard(1)

	for !j.AtEnd() {
		c, _, err := j.r.ReadRune()
		if err != nil {
			return nil, err
		}

		switch c {
		case '\\':
			c2, _, err := j.r.ReadRune()
			if err != nil {
				return nil, err
			}
			switch c2 {
			case 'u':
				hex, err := j.nextUnicodeChar()
				if err != nil {
					return nil, err
				}
				buf.WriteRune(rune(hex))

			case 'n':
				buf.WriteRune('\n')
			case 't':
				buf.WriteRune('\t')
			case 'r':
				buf.WriteRune('\r')
			case '\\':
				buf.WriteRune('\\')
			case 'f':
				buf.WriteRune('\f')
			case 'b':
				buf.WriteRune('\b')
			case '"':
				buf.WriteRune('"')
			default:
				return nil, fmt.Errorf("unexpected character '\\%c'", c2)
			}

		case '"':
			// Done
			t := j.newToken(String)
			t.StringValue = buf.String()
			return t, nil

		default:
			buf.WriteRune(c)
		}
	}

	return nil, fmt.Errorf("unexpected end of string")
}

func (j *StreamingJsonTokenizer) nextNumber() (*Token, error) {
	neg := false
	c, err := j.peekRune()
	if err != nil {
		return nil, err
	}

	if c == '-' {
		neg = true
		j.r.Discard(1)
	}

	base, err := j.nextNumbers()
	if err != nil {
		return nil, err
	}

	c, err = j.peekRune()
	if err != nil {
		return nil, err
	}
	if c == '.' {
		j.r.Discard(1)
		fraction, err := j.nextNumbers()
		if err != nil {
			return nil, err
		}

		float, err := strconv.ParseFloat(fmt.Sprintf("%s.%s", base, fraction), 64)
		if err != nil {
			return nil, err
		}

		if neg {
			float = -float
		}

		tok := j.newToken(Float)
		tok.FloatValue = float
		return tok, nil
	} else {
		n, err := strconv.ParseInt(base, 10, 64)
		if err != nil {
			return nil, err
		}
		if neg {
			n = -n
		}
		tok := j.newToken(Int)
		tok.IntValue = int(n)
		return tok, nil
	}
}

func (j *StreamingJsonTokenizer) peekRune() (rune, error) {
	val, err := j.r.Peek(1)
	if err == io.EOF {
		return 0, nil
	} else if err != nil {
		return 0, err
	} else {
		return rune(val[0]), nil
	}
}

func (j *StreamingJsonTokenizer) nextNumbers() (string, error) {
	buff := strings.Builder{}

	for {
		c, err := j.peekRune()
		if err == io.EOF {
			return buff.String(), nil
		} else if err != nil {
			return "", err
		}
		if c >= '0' && c <= '9' {
			buff.WriteRune(c)
			j.r.Discard(1)
		} else {
			break
		}
	}

	return buff.String(), nil
}

func (j *StreamingJsonTokenizer) nextUnicodeChar() (int, error) {
	i := 0
	n := 0
	for n < 4 {
		c, err := j.peekRune()
		if err == io.EOF {
			return i, nil
		} else if err != nil {
			return 0, err
		}
		if c >= '0' && c <= '9' {
			i = (i << 4) + int(c-'0')
			j.r.Discard(1)
		} else if c >= 'a' && c <= 'f' {
			i = (i << 4) + int(c-'a'+10)
			j.r.Discard(1)
		} else if c >= 'A' && c <= 'F' {
			i = (i << 4) + int(c-'A'+10)
			j.r.Discard(1)
		} else {
			break
		}
		n++
	}

	return i, nil
}

func isWhitespace(c rune) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r'
}
