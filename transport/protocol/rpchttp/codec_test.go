// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpchttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

func TestNewConfiguredMarshalers_NilConfig(t *testing.T) {
	set, err := newConfiguredMarshalers(nil)
	require.NoError(t, err)
	assert.Nil(t, set.inbound)
	assert.Nil(t, set.outbound)
}

func TestNewConfiguredMarshalers_ValidConfig(t *testing.T) {
	cfg := &MarshalerConfigSet{
		Inbound: &MarshalerConfig{
			Type:   marshaler.SchemeJSONPb,
			Config: nil,
		},
		Outbound: &MarshalerConfig{
			Type:   marshaler.SchemeJSONPb,
			Config: nil,
		},
	}
	set, err := newConfiguredMarshalers(cfg)
	require.NoError(t, err)
	assert.NotNil(t, set.inbound)
	assert.NotNil(t, set.outbound)
	assert.IsType(t, &marshaler.JSONPb{}, set.inbound)
	assert.IsType(t, &marshaler.JSONPb{}, set.outbound)
}

func TestBuildConfiguredMarshaler_NilConfig(t *testing.T) {
	m, err := buildConfiguredMarshaler(nil)
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestBuildConfiguredMarshaler_ValidConfig(t *testing.T) {
	cfg := &MarshalerConfig{
		Type:   marshaler.SchemeJSONPb,
		Config: nil,
	}
	m, err := buildConfiguredMarshaler(cfg)
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.IsType(t, &marshaler.JSONPb{}, m)
}

func TestSelectInboundMarshaler_Configured(t *testing.T) {
	configured := marshaler.NewJSONPbMarshalerWithConfig(nil)
	result := selectInboundMarshaler(configured, "text/plain")
	assert.Equal(t, configured, result)
}

func TestSelectInboundMarshaler_ContentType(t *testing.T) {
	result := selectInboundMarshaler(nil, marshaler.ContentTypeJSON)
	assert.NotNil(t, result)
	assert.IsType(t, &marshaler.JSONPb{}, result)
}

func TestSelectInboundMarshaler_Fallback(t *testing.T) {
	// When no configured marshaler and unrecognized content type, fall back to default.
	result := selectInboundMarshaler(nil, "text/plain")
	assert.NotNil(t, result)
}

func TestSelectOutboundMarshaler_Configured(t *testing.T) {
	configured := marshaler.NewJSONPbMarshalerWithConfig(nil)
	result := selectOutboundMarshaler(configured, "text/plain", nil)
	assert.Equal(t, configured, result)
}

func TestSelectOutboundMarshaler_Accept(t *testing.T) {
	result := selectOutboundMarshaler(nil, marshaler.ContentTypeJSON, nil)
	assert.NotNil(t, result)
	assert.IsType(t, &marshaler.JSONPb{}, result)
}

func TestSelectOutboundMarshaler_EmptyAcceptFallback(t *testing.T) {
	fallback := marshaler.NewJSONPbMarshalerWithConfig(nil)
	result := selectOutboundMarshaler(nil, "", fallback)
	assert.Equal(t, fallback, result)
}

func TestSelectOutboundMarshaler_NoMatchFallback(t *testing.T) {
	fallback := marshaler.NewJSONPbMarshalerWithConfig(nil)
	result := selectOutboundMarshaler(nil, "text/xml", fallback)
	assert.NotNil(t, result)
}

func TestSelectOutboundMarshaler_NilBoth(t *testing.T) {
	// When configured is nil, accept is not empty but unmatched, and fallback is nil,
	// should return the default marshaler.
	result := selectOutboundMarshaler(nil, "text/xml", nil)
	assert.NotNil(t, result)
}

func TestSelectOutboundMarshaler_EmptyAcceptNilFallback(t *testing.T) {
	// When accept is empty and fallback is nil, should return default marshaler.
	result := selectOutboundMarshaler(nil, "", nil)
	assert.NotNil(t, result)
}

func TestNewConfiguredMarshalersWithBuilders_NonNil(t *testing.T) {
	custom := marshaler.NewJSONPbMarshalerWithConfig(nil)
	builders := map[string]marshaler.MarshalerBuilder{
		"custom": func() (marshaler.Marshaler, error) { return custom, nil },
	}
	cfg := &MarshalerConfigSet{
		Inbound: &MarshalerConfig{
			Type:   "custom",
			Config: nil,
		},
		Outbound: &MarshalerConfig{
			Type:   "custom",
			Config: nil,
		},
	}
	set, err := newConfiguredMarshalersWithBuilders(builders, cfg)
	require.NoError(t, err)
	assert.Equal(t, custom, set.inbound)
	assert.Equal(t, custom, set.outbound)
}

func TestBuildConfiguredMarshalerWithBuilders_InvalidType(t *testing.T) {
	cfg := &MarshalerConfig{
		Type:   "nonexistent-type",
		Config: nil,
	}
	m, err := buildConfiguredMarshalerWithBuilders(nil, cfg)
	require.Error(t, err)
	assert.Nil(t, m)
}
