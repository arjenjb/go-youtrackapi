package jsoncodec

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
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
	var c rune
	for {
		peek, err := j.r.Peek(1)
		if err == io.EOF {
			return j.newToken(EOF), nil
		} else if err != nil {
			return nil, err
		}
		c = rune(peek[0])
		if !isWhitespace(c) {
			break
		}

		if _, err := j.r.Discard(1); err != nil {
			return nil, err
		}
	}

	if c >= '0' && c <= '9' {
		return j.nextNumber()
	}

	switch rune(c) {
	case '[':
		_, err := j.r.Discard(1)
		return &Token{Type: ArrayBegin}, err
	case ']':
		_, err := j.r.Discard(1)
		return &Token{Type: ArrayClose}, err
	case '{':
		_, err := j.r.Discard(1)
		return &Token{Type: ObjectBegin}, err
	case '}':
		_, err := j.r.Discard(1)
		return &Token{Type: ObjectClose}, err
	case ',':
		_, err := j.r.Discard(1)
		return &Token{Type: Comma}, err
	case ':':
		_, err := j.r.Discard(1)
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
				value := rune(hex)
				if 0xD800 <= value && value <= 0xDBFF {
					slash, _, err := j.r.ReadRune()
					if err != nil {
						return nil, err
					}
					u, _, err := j.r.ReadRune()
					if err != nil {
						return nil, err
					}
					if slash != '\\' || u != 'u' {
						return nil, fmt.Errorf("expected low surrogate after high surrogate")
					}
					low, err := j.nextUnicodeChar()
					if err != nil {
						return nil, err
					}
					if low < 0xDC00 || low > 0xDFFF {
						return nil, fmt.Errorf("expected low surrogate after high surrogate")
					}
					value = utf16.DecodeRune(value, rune(low))
				}
				buf.WriteRune(value)

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
	literal := strings.Builder{}
	c, err := j.peekRune()
	if err != nil {
		return nil, err
	}

	if c == '-' {
		literal.WriteRune(c)
		j.r.Discard(1)
	}

	base, err := j.nextNumbers()
	if err != nil {
		return nil, err
	}
	if base == "" {
		return nil, fmt.Errorf("expected digit in number")
	}
	if len(base) > 1 && base[0] == '0' {
		return nil, fmt.Errorf("leading zero in number")
	}
	literal.WriteString(base)

	c, err = j.peekRune()
	if err != nil {
		return nil, err
	}
	isFloat := false
	if c == '.' {
		isFloat = true
		literal.WriteRune(c)
		j.r.Discard(1)
		fraction, err := j.nextNumbers()
		if err != nil {
			return nil, err
		}
		if fraction == "" {
			return nil, fmt.Errorf("expected digit after decimal point")
		}
		literal.WriteString(fraction)
	}

	c, err = j.peekRune()
	if err != nil {
		return nil, err
	}
	if c == 'e' || c == 'E' {
		isFloat = true
		literal.WriteRune(c)
		j.r.Discard(1)

		c, err = j.peekRune()
		if err != nil {
			return nil, err
		}
		if c == '+' || c == '-' {
			literal.WriteRune(c)
			j.r.Discard(1)
		}

		exponent, err := j.nextNumbers()
		if err != nil {
			return nil, err
		}
		if exponent == "" {
			return nil, fmt.Errorf("expected digit in exponent")
		}
		literal.WriteString(exponent)
	}

	if isFloat {
		float, err := strconv.ParseFloat(literal.String(), 64)
		if err != nil {
			return nil, err
		}
		tok := j.newToken(Float)
		tok.FloatValue = float
		return tok, nil
	}

	n, err := strconv.ParseInt(literal.String(), 10, 64)
	if err != nil {
		return nil, err
	}
	tok := j.newToken(Int)
	tok.IntValue = int(n)
	return tok, nil
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
	for n := 0; n < 4; n++ {
		c, _, err := j.r.ReadRune()
		if err != nil {
			return 0, fmt.Errorf("incomplete unicode escape: %w", err)
		}
		if c >= '0' && c <= '9' {
			i = (i << 4) + int(c-'0')
		} else if c >= 'a' && c <= 'f' {
			i = (i << 4) + int(c-'a'+10)
		} else if c >= 'A' && c <= 'F' {
			i = (i << 4) + int(c-'A'+10)
		} else {
			return 0, fmt.Errorf("invalid character %q in unicode escape", c)
		}
	}

	return i, nil
}

func isWhitespace(c rune) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r'
}
