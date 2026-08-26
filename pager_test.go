package youtrackapi_test

import (
	"context"
	"errors"
	"testing"

	youtrackapi "github.com/arjenjb/go-youtrackapi"
	"github.com/stretchr/testify/require"
)

type pagerRequest struct {
	Top  int
	Skip int
}

func TestPagerAllFetchesEveryPage(t *testing.T) {
	items := make([]int, 43)
	for i := range items {
		items[i] = i
	}

	var requests []pagerRequest
	fetch := func(_ context.Context, request pagerRequest) ([]int, error) {
		requests = append(requests, request)
		if request.Skip >= len(items) {
			return nil, nil
		}

		end := min(request.Skip+request.Top, len(items))
		return items[request.Skip:end], nil
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)
	got, err := pager.All(context.Background())

	require.NoError(t, err)
	require.Equal(t, items, got)
	require.Equal(t, []pagerRequest{{Top: 42, Skip: 0}, {Top: 42, Skip: 42}}, requests)
}

func TestPagerNextStopsAfterAnEmptyPage(t *testing.T) {
	calls := 0
	fetch := func(_ context.Context, request pagerRequest) ([]string, error) {
		calls++
		require.Equal(t, pagerRequest{Top: 42, Skip: 0}, request)
		return nil, nil
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)

	item, ok, err := pager.Next(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, item)

	item, ok, err = pager.Next(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, item)
	require.Equal(t, 1, calls)
}

func TestPagerAllPropagatesFetchError(t *testing.T) {
	wantErr := errors.New("fetch failed")
	fetch := func(_ context.Context, _ pagerRequest) ([]int, error) {
		return nil, wantErr
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)
	items, err := pager.All(context.Background())

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, items)
}

func TestPagerNextRetriesTheSamePageAfterFetchError(t *testing.T) {
	wantErr := errors.New("temporary fetch failure")
	var requests []pagerRequest
	fetch := func(_ context.Context, request pagerRequest) ([]int, error) {
		requests = append(requests, request)
		if len(requests) == 1 {
			return []int{-1, -2}, wantErr
		}

		return []int{7}, nil
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)

	item, ok, err := pager.Next(context.Background())
	require.ErrorIs(t, err, wantErr)
	require.False(t, ok)
	require.Nil(t, item)

	item, ok, err = pager.Next(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 7, *item)
	require.Equal(t, []pagerRequest{{Top: 42, Skip: 0}, {Top: 42, Skip: 0}}, requests)
}

func TestPagerAllProbesForAnotherPageAfterAnExactPage(t *testing.T) {
	items := make([]int, 42)
	for i := range items {
		items[i] = i
	}

	var requests []pagerRequest
	fetch := func(_ context.Context, request pagerRequest) ([]int, error) {
		requests = append(requests, request)
		if request.Skip == 0 {
			return items, nil
		}

		return nil, nil
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)
	got, err := pager.All(context.Background())

	require.NoError(t, err)
	require.Equal(t, items, got)
	require.Equal(t, []pagerRequest{{Top: 42, Skip: 0}, {Top: 42, Skip: 42}}, requests)

	item, ok, err := pager.Next(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, item)
	require.Len(t, requests, 2)
}

func TestPagerNextPropagatesContextAndCancellation(t *testing.T) {
	type contextKey string
	const requestIDKey contextKey = "request-id"

	ctx := context.WithValue(context.Background(), requestIDKey, "request-123")
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	fetch := func(fetchCtx context.Context, _ pagerRequest) ([]int, error) {
		require.Equal(t, "request-123", fetchCtx.Value(requestIDKey))
		require.ErrorIs(t, fetchCtx.Err(), context.Canceled)
		return nil, fetchCtx.Err()
	}

	request := pagerRequest{}
	pager := youtrackapi.NewPager(fetch, &request)
	item, ok, err := pager.Next(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
	require.Nil(t, item)
}
