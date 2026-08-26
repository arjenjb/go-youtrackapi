package youtrackapi

import (
	"context"
	"reflect"
)

type Pager[R any, T any] struct {
	f       func(context.Context, R) ([]T, error)
	request *R

	top  int
	skip int

	atEnd  bool
	i      int
	result []T
}

func (t *Pager[R, T]) Next(ctx context.Context) (*T, bool, error) {
	if t.atEnd {
		return nil, false, nil
	}

	if t.i+1 >= len(t.result) {
		if len(t.result) > 0 && t.i < (t.top-1) {
			// no more items to fetch
			t.atEnd = true
			return nil, false, nil
		}

		// There's more
		t.nextRequest()

		var err error
		t.result, err = t.f(ctx, *t.request)
		if err != nil {
			return nil, false, err
		}
		if len(t.result) == 0 {
			t.atEnd = true
			return nil, false, nil
		}

		t.i = 0
	} else {
		t.i++
	}

	return &t.result[t.i], true, nil
}

func (t *Pager[R, T]) nextRequest() {
	r := reflect.ValueOf(t.request)
	r.Elem().FieldByName("Top").Set(reflect.ValueOf(t.top))
	r.Elem().FieldByName("Skip").Set(reflect.ValueOf(t.skip))

	t.skip += t.top
}

func (t *Pager[R, T]) All(ctx context.Context) ([]T, error) {
	var result []T
	for {
		item, ok, err := t.Next(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		result = append(result, *item)
	}

	return result, nil
}

func NewPager[R any, T any](f func(ctx context.Context, r R) ([]T, error), req *R) Pager[R, T] {
	return Pager[R, T]{f: f, request: req, top: 42}
}
