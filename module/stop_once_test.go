package module

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopOnceReturnsFirstError(t *testing.T) {
	var stop StopOnce
	calls := 0
	err := stop.Do(context.Background(), func(context.Context) error {
		calls++
		return errors.New("first")
	})
	require.EqualError(t, err, "first")

	err = stop.Do(context.Background(), func(context.Context) error {
		calls++
		return nil
	})
	require.EqualError(t, err, "first")
	require.Equal(t, 1, calls)
}

func TestStopOnceRecoversPanic(t *testing.T) {
	var stop StopOnce
	err := stop.Do(context.Background(), func(context.Context) error {
		panic("boom")
	})
	require.ErrorContains(t, err, "stop panic recovered")
	require.ErrorContains(t, err, "boom")
}
