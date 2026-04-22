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

package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding"
	jsonrawenc "github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding/jsonraw"
	rawenc "github.com/codesjoy/yggdrasil/v3/remote/protocol/grpc/encoding/raw"
)

type testCodec struct {
	name string
}

func (c testCodec) Marshal(v interface{}) ([]byte, error) {
	return nil, nil
}

func (c testCodec) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func (c testCodec) Name() string {
	return c.name
}

func TestCallContentSubtypeSelectsRegisteredCodec(t *testing.T) {
	ctx := WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name))
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	require.NoError(t, setCallInfoCodec(info))
	require.NotNil(t, info.codec)
	assert.Equal(t, rawenc.Name, info.contentSubtype)
	assert.Equal(t, rawenc.Name, info.codec.Name())
}

func TestCallContentSubtypeSelectsJSONRawCodec(t *testing.T) {
	ctx := WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name))
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	require.NoError(t, setCallInfoCodec(info))
	require.NotNil(t, info.codec)
	assert.Equal(t, jsonrawenc.Name, info.contentSubtype)
	assert.Equal(t, jsonrawenc.Name, info.codec.Name())
}

func TestForceCodecUsesCodecNameAsContentSubtype(t *testing.T) {
	ctx := WithCallOptions(context.Background(), ForceCodec(testCodec{name: "custom"}))
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	require.NoError(t, setCallInfoCodec(info))
	require.NotNil(t, info.codec)
	assert.Equal(t, "custom", info.contentSubtype)
	assert.Equal(t, "custom", info.codec.Name())
}

func TestExplicitContentSubtypeWinsOverForcedCodecName(t *testing.T) {
	forced := testCodec{name: "forced"}
	ctx := WithCallOptions(
		context.Background(),
		ForceCodec(forced),
		CallContentSubtype(rawenc.Name),
	)
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	require.NoError(t, setCallInfoCodec(info))
	require.NotNil(t, info.codec)
	assert.Equal(t, rawenc.Name, info.contentSubtype)
	assert.Equal(t, forced.Name(), info.codec.Name())
}

func TestForceCodecRejectsEmptyEffectiveSubtype(t *testing.T) {
	ctx := WithCallOptions(context.Background(), ForceCodec(testCodec{}))
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	err := setCallInfoCodec(info)
	require.Error(t, err)
	assert.ErrorContains(t, err, "non-empty content-subtype")
}

func TestForceJSONRawCodecUsesCodecNameAsContentSubtype(t *testing.T) {
	ctx := WithCallOptions(context.Background(), ForceCodec(getCodecOrPanic(jsonrawenc.Name)))
	info := defaultCallInfo()

	require.NoError(t, applyCallOptions(info, callOptionsFromContext(ctx)))
	require.NoError(t, setCallInfoCodec(info))
	require.NotNil(t, info.codec)
	assert.Equal(t, jsonrawenc.Name, info.contentSubtype)
	assert.Equal(t, jsonrawenc.Name, info.codec.Name())
}

func getCodecOrPanic(name string) encoding.Codec {
	c := encoding.GetCodec(name)
	if c == nil {
		panic("codec not registered: " + name)
	}
	return c
}
