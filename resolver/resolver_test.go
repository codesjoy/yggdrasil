package resolver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testResolver struct{ kind string }

func (t *testResolver) AddWatch(string, Client) error { return nil }
func (t *testResolver) DelWatch(string, Client) error { return nil }
func (t *testResolver) Type() string                  { return t.kind }

func TestGetDefaultResolverWithoutConfigReturnsNil(t *testing.T) {
	r, err := Get(DefaultResolverName)
	require.NoError(t, err)
	require.Nil(t, r)
}

func TestGetResolverFromDiscoveryTree(t *testing.T) {
	RegisterBuilder("mock", func(name string) (Resolver, error) {
		return &testResolver{kind: name}, nil
	})
	Configure(map[string]Spec{"demo": {Type: "mock"}})

	r, err := Get("demo")
	require.NoError(t, err)
	require.Equal(t, "demo", r.Type())
}
