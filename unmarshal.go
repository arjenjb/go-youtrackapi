package youtrackapi

import (
	"fmt"
	"time"
)

func determineTypeDiscriminator(r *JSONReader, s string) (string, error) {
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

func marshalstring(w *JsonMarshaler, s string) error {
	return w.WriteString(s)
}

func marshalbool(w *JsonMarshaler, value bool) error {
	return w.WriteBool(value)
}

func marshalint(w *JsonMarshaler, value int) error {
	return w.WriteInt(value)
}

func marshalfloat64(w *JsonMarshaler, value float64) error {
	return w.WriteFloat64(value)
}

func marshalany(w *JsonMarshaler, value any) error {
	return w.WriteValue(value)
}

func marshalTime(w *JsonMarshaler, t time.Time) error {
	return w.WriteInt(int(t.UnixMilli()))
}

func unmarshalTime(r *JSONReader) (*time.Time, error) {
	i, err := r.NextOptionalInt()
	if err != nil {
		return nil, err
	} else if i == nil {
		return nil, nil
	}

	t := time.UnixMilli(int64(*i)).UTC()
	return &t, nil
}

func unmarshalAbstractList[T any](r *JSONReader, f func(r *JSONReader) (T, error)) ([]T, error) {
	var result []T
	err := r.NextListDo(func(r *JSONReader) error {
		el, err := f(r)
		if err != nil {
			return err
		}
		result = append(result, el)
		return nil
	})

	return result, err
}

func marshalList[T any](w *JsonMarshaler, coll []T, f func(*JsonMarshaler, T) error) error {
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

func unmarshalList[T any](r *JSONReader, f func(*JSONReader) (*T, error)) ([]T, error) {
	var result []T
	err := r.NextListDo(func(r *JSONReader) error {
		el, err := f(r)
		if err != nil {
			return err
		}
		result = append(result, *el)
		return nil
	})

	return result, err
}
