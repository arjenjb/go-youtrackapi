package jsoncodec

import (
	"bufio"
	"fmt"
	"io"

	"github.com/arjenjb/go-youtrackapi/internal/util"
)

type JsonTokenizerInterface interface {
	Next() (*Token, error)
}

type PeekableTokenizerInterface interface {
	Next() (*Token, error)
	Peek() (*Token, error)
}

func (j *JsonBufferingTokenizer) fillBuffer() error {
	if j.next != nil {
		return nil
	}

	t, err := j.tok.Next()
	if err != nil {
		return err
	}
	j.next = t
	return nil
}

func (j *JsonBufferingTokenizer) Peek() (*Token, error) {
	err := j.fillBuffer()
	if err != nil {
		return nil, err
	}

	return j.next, nil
}

func (j *JsonBufferingTokenizer) Next() (*Token, error) {
	var t *Token
	if j.next != nil {
		t = j.next
		j.next = nil
	} else {
		var err error
		t, err = j.tok.Next()
		if err != nil {
			return nil, err
		}
	}

	err := j.fillBuffer()
	if err != nil {
		return nil, err
	}

	return t, nil
}

type JsonBufferingTokenizer struct {
	tok  JsonTokenizerInterface
	next *Token
}

type RewindableJsonTokenizer struct {
	marked bool
	buffer util.LinkedList[*Token]
	tok    PeekableTokenizerInterface
}

func (b *RewindableJsonTokenizer) Next() (*Token, error) {
	if !b.marked && b.buffer.IsEmpty() {
		return b.tok.Next()
	} else if !b.marked {
		tok := b.buffer.RemoveFirst()
		return tok, nil
	} else {
		t, err := b.tok.Next()
		if err != nil {
			return nil, err
		}
		b.buffer.Append(t)
		return t, nil
	}
}

func (b *RewindableJsonTokenizer) Peek() (*Token, error) {
	if !b.marked && b.buffer.IsEmpty() {
		return b.tok.Peek()
	} else if !b.marked {
		return b.buffer.First(), nil
	} else {
		return b.tok.Peek()
	}
}

func (b *RewindableJsonTokenizer) Buffer() {
	if b.marked {
		panic("Cannot mark more than two places")
	}
	b.marked = true
}

func (b *RewindableJsonTokenizer) Rewind() {
	if !b.marked {
		panic("Cannot rewind, because we're not buffering")
	}
	b.marked = false
}

func NewBufferingJsonTokenizer(tok PeekableTokenizerInterface) *RewindableJsonTokenizer {
	return &RewindableJsonTokenizer{
		tok:    tok,
		marked: false,
		buffer: util.LinkedList[*Token]{},
	}
}

type Reader struct {
	t *RewindableJsonTokenizer
}

func NewReader(r io.Reader) *Reader {
	tok := NewBufferingJsonTokenizer(
		&JsonBufferingTokenizer{
			tok: &StreamingJsonTokenizer{
				r: bufio.NewReader(r),
			}})

	return &Reader{
		t: tok,
	}
}

func (j *Reader) Buffer() {
	j.t.Buffer()
}

func (j *Reader) Rewind() {
	j.t.Rewind()
}

func (j *Reader) Next() (*Token, error) {
	t, err := j.t.Next()
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (j *Reader) Expect(begin TokenType) (*Token, error) {
	t, err := j.Next()
	if err != nil {
		return nil, err
	}
	if t.Type != begin {
		return nil, fmt.Errorf("unexpected token type: %v", t.Type)
	}

	return t, nil
}

func (j *Reader) NextObjectDo(f func(key string, r *Reader) error) error {
	if _, err := j.Expect(ObjectBegin); err != nil {
		return err
	}

	// Check if this is an empty object
	tkn, err := j.Peek()
	if err != nil {
		return err
	}
	if tkn.Type == ObjectClose {
		j.Next()
		return nil
	}

	for {
		tkn, err := j.Expect(String)
		if err != nil {
			return err
		}
		if _, err := j.Expect(Colon); err != nil {
			return err
		}

		err = f(tkn.StringValue, j)
		if err != nil {
			return err
		}

		tkn, err = j.Peek()
		if err != nil {
			return err
		} else if tkn.Type != Comma {
			break
		} else {
			j.Next()
		}
	}

	if _, err := j.Expect(ObjectClose); err != nil {
		return err
	}

	return nil
}

func (j *Reader) NextOptionalString() (*string, error) {
	tkn, err := j.Next()
	if err != nil {
		return nil, err
	} else if tkn.Type == Null {
		return nil, nil
	} else if tkn.Type == String {
		return &tkn.StringValue, nil
	} else {
		return nil, fmt.Errorf("unexpected token type: %d", tkn.Type)
	}
}

func (j *Reader) NextOptionalFloat64() (*float64, error) {
	tkn, err := j.Next()
	if err != nil {
		return nil, err
	} else if tkn.Type == Null {
		return nil, nil
	} else if tkn.Type == Float {
		return &tkn.FloatValue, nil
	} else if tkn.Type == Int {
		value := float64(tkn.IntValue)
		return &value, nil
	} else {
		return nil, fmt.Errorf("unexpected token type: %d", tkn.Type)
	}
}

func (j *Reader) NextOptionalValue() (*any, error) {
	value, err := j.NextValue()
	if err != nil || value == nil {
		return nil, err
	}
	return &value, nil
}

func (j *Reader) NextValue() (interface{}, error) {
	tkn, err := j.Peek()
	if err != nil {
		return nil, err
	}
	switch tkn.Type {
	case Bool:
		j.Next()
		return tkn.BoolValue, nil
	case Null:
		j.Next()
		return nil, nil
	case String:
		j.Next()
		return tkn.StringValue, nil
	case Int:
		j.Next()
		return tkn.IntValue, nil
	case Float:
		j.Next()
		return tkn.FloatValue, nil
	case ArrayBegin:
		return j.NextList()
	case ObjectBegin:
		return j.NextObject()
	case EOF:
		return nil, fmt.Errorf("unexpected EOF")
	default:
		return nil, fmt.Errorf("unexpected token type: %d", tkn.Type)
	}
}

func (j *Reader) NextObject() (map[string]interface{}, error) {
	result := map[string]interface{}{}
	err := j.NextObjectDo(func(key string, r *Reader) error {
		val, err := r.NextValue()
		if err != nil {
			return err
		}
		result[key] = val
		return nil
	})
	return result, err
}

func (j *Reader) NextList() ([]interface{}, error) {
	result := make([]interface{}, 0)
	err := j.NextListDo(func(r *Reader) error {
		val, err := r.NextValue()
		if err != nil {
			return err
		}
		result = append(result, val)
		return nil
	})

	return result, err
}

func (j *Reader) NextListDo(f func(r *Reader) error) error {
	if _, err := j.Expect(ArrayBegin); err != nil {
		return err
	}

	// Check if this is an empty array
	tkn, err := j.Peek()
	if err != nil {
		return err
	}
	if tkn.Type == ArrayClose {
		j.Next()
		return nil
	}

	for {
		err := f(j)
		if err != nil {
			return err
		}

		n, err := j.Next()
		if err != nil {
			return err
		}

		if n.Type == Comma {
			// continue
		} else if n.Type == ArrayClose {
			return nil
		} else {
			return fmt.Errorf("unexpected token %v", n)
		}
	}
}

func (j *Reader) NextOptionalBool() (*bool, error) {
	tkn, err := j.Next()
	if err != nil {
		return nil, err
	} else if tkn.Type == Null {
		return nil, nil
	} else if tkn.Type == Bool {
		return &tkn.BoolValue, nil
	} else {
		return nil, fmt.Errorf("unexpected token type: %d", tkn.Type)
	}
}

func (j *Reader) NextOptionalInt() (*int, error) {
	tkn, err := j.Next()
	if err != nil {
		return nil, err
	} else if tkn.Type == Null {
		return nil, nil
	} else if tkn.Type == Int {
		return &tkn.IntValue, nil
	} else {
		return nil, fmt.Errorf("unexpected token type: %d", tkn.Type)
	}
}

func (j *Reader) Peek() (*Token, error) {
	return j.t.Peek()
}
