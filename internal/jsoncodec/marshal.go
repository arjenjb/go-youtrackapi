package jsoncodec

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/arjenjb/go-youtrackapi/internal/util"
)

type State uint8

const (
	StateBegin State = iota
	StateArray
	StateObject
	StateObjectKey
	StateValue
)

type Marshaler struct {
	stack util.LinkedList[State]
	state State
	w     io.Writer
}

func NewMarshaler(w io.Writer) *Marshaler {
	return &Marshaler{
		stack: util.LinkedList[State]{},
		state: StateBegin,
		w:     w,
	}
}

func (m *Marshaler) WriteString(s string) error {
	if err := m.newValue(); err != nil {
		return err
	}
	m.state = StateValue
	return m.writeString(s)
}

func (m *Marshaler) WriteBool(b bool) error {
	if err := m.newValue(); err != nil {
		return err
	}
	m.state = StateValue

	if b {
		_, err := m.w.Write([]byte("true"))
		return err
	} else {
		_, err := m.w.Write([]byte("false"))
		return err
	}
}

func (m *Marshaler) WriteNull() error {
	if err := m.newValue(); err != nil {
		return err
	}
	m.state = StateValue

	_, err := m.w.Write([]byte("null"))
	return err
}

func (m *Marshaler) WriteInt(i int) error {
	return m.writeValue(i)
}

func (m *Marshaler) WriteFloat64(f float64) error {
	return m.writeValue(f)
}

func (m *Marshaler) WriteValue(value any) error {
	return m.writeValue(value)
}

func (m *Marshaler) ObjectStart() error {
	if err := m.newValue(); err != nil {
		return err
	}

	_, err := m.w.Write([]byte("{"))
	if err != nil {
		return err
	}
	m.stack.Append(StateObject)
	m.state = StateBegin
	return nil
}

func (m *Marshaler) ObjectEnd() error {
	if m.stack.IsEmpty() || m.stack.Tail.E != StateObject || m.state == StateObjectKey {
		return fmt.Errorf("unexpected object end token")
	}

	_, err := m.w.Write([]byte("}"))
	if err != nil {
		return err
	}

	m.stack.RemoveLast()
	m.state = StateValue
	return nil
}

func (m *Marshaler) ArrayStart() error {
	if err := m.newValue(); err != nil {
		return err
	}
	_, err := m.w.Write([]byte("["))
	m.stack.Append(StateArray)
	m.state = StateBegin
	return err
}

func (m *Marshaler) ArrayEnd() error {
	if m.stack.IsEmpty() || m.stack.Tail.E != StateArray {
		return fmt.Errorf("unexpected array end token")
	}

	_, err := m.w.Write([]byte("]"))
	if err != nil {
		return err
	}
	m.stack.RemoveLast()
	m.state = StateValue
	return nil
}

func (m *Marshaler) WriteKey(s string) error {
	if m.stack.IsEmpty() || m.stack.Tail.E != StateObject {
		return fmt.Errorf("unexpected object key")
	}

	if m.state == StateBegin {
		// Okay, just emit the key
	} else if m.state == StateValue {
		// A value has been emitted, so write a comma
		_, err := m.w.Write([]byte(","))
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unexpected state")
	}

	// write the key
	err := m.writeString(s)
	if err != nil {
		return err
	}
	_, err = m.w.Write([]byte(":"))
	if err != nil {
		return err
	}

	m.state = StateObjectKey
	return nil
}

// Write a basic value, string, bool, nil, float or int
func (m *Marshaler) newValue() error {
	if m.stack.IsEmpty() {
		switch m.state {
		case StateBegin:
			m.state = StateValue
			return nil
		default:
			return fmt.Errorf("unexpected state: %v", m.state)
		}
	} else {
		switch m.stack.Last() {
		case StateObject:
			switch m.state {
			case StateObjectKey:
				m.state = StateValue
				return nil

			default:
				return fmt.Errorf("unexpected state: %v", m.state)
			}

		case StateArray:
			switch m.state {
			case StateBegin:
				m.state = StateValue
				return nil
			case StateValue:
				_, err := m.w.Write([]byte(","))
				return err
			default:
				return fmt.Errorf("unexpected state: %v", m.state)
			}
		default:
			return fmt.Errorf("unexpected state: %v", m.state)
		}
	}
}

func (m *Marshaler) writeValue(val any) error {
	if err := m.newValue(); err != nil {
		return err
	}
	m.state = StateValue
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	_, err = m.w.Write(data)
	return err
}

func (m *Marshaler) writeString(s string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = m.w.Write(data)
	return err
}
