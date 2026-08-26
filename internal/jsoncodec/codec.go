package jsoncodec

import (
	"fmt"
	"time"
)

func DetermineTypeDiscriminator(r *Reader, s string) (string, error) {
	r.Buffer()
	defer r.Rewind()

	// We expect an object
	_, err := r.Expect(ObjectBegin)
	if err != nil {
		return "", err
	}

	for {
		n, err := r.Next()
		if err != nil {
			return "", err
		}

		if n.Type == EOF {
			return "", fmt.Errorf("unexpected end of JSON input")
		} else if n.Type != String {
			return "", fmt.Errorf("expected an object key")
		}

		key := n.StringValue

		// Get the colon
		_, err = r.Expect(Colon)
		if err != nil {
			return "", err
		}

		if key != s {
			// Consume the whole value
			_, err = r.NextValue()
			if err != nil {
				return "", err
			}
		} else {
			// Found it
			d, err := r.Expect(String)
			if err != nil {
				return "", nil
			}

			return d.StringValue, nil
		}

		t, err := r.Next()
		if err != nil {
			return "", err
		}

		if t.Type == Comma {
			// Okay, just continue
		} else if t.Type == ObjectClose {
			return "", fmt.Errorf("did not find type discriminator")
		} else {
			return "", fmt.Errorf("expected token")
		}
	}
}

func MarshalString(w *Marshaler, s string) error {
	return w.WriteString(s)
}

func MarshalBool(w *Marshaler, value bool) error {
	return w.WriteBool(value)
}

func MarshalInt(w *Marshaler, value int) error {
	return w.WriteInt(value)
}

func MarshalFloat64(w *Marshaler, value float64) error {
	return w.WriteFloat64(value)
}

func MarshalAny(w *Marshaler, value any) error {
	return w.WriteValue(value)
}

func MarshalTime(w *Marshaler, t time.Time) error {
	return w.WriteInt(int(t.UnixMilli()))
}

func UnmarshalTime(r *Reader) (*time.Time, error) {
	i, err := r.NextOptionalInt()
	if err != nil {
		return nil, err
	} else if i == nil {
		return nil, nil
	}

	t := time.UnixMilli(int64(*i)).UTC()
	return &t, nil
}

func UnmarshalAbstractList[T any](r *Reader, f func(r *Reader) (T, error)) ([]T, error) {
	var result []T
	err := r.NextListDo(func(r *Reader) error {
		el, err := f(r)
		if err != nil {
			return err
		}
		result = append(result, el)
		return nil
	})

	return result, err
}

func MarshalList[T any](w *Marshaler, coll []T, f func(*Marshaler, T) error) error {
	err := w.ArrayStart()
	if err != nil {
		return err
	}

	for _, each := range coll {
		if err = f(w, each); err != nil {
			return err
		}
	}
	return w.ArrayEnd()
}

func UnmarshalList[T any](r *Reader, f func(*Reader) (*T, error)) ([]T, error) {
	var result []T
	err := r.NextListDo(func(r *Reader) error {
		el, err := f(r)
		if err != nil {
			return err
		}
		result = append(result, *el)
		return nil
	})

	return result, err
}
