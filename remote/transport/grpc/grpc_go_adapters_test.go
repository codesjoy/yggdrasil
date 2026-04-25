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
	"io"
	"testing"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	gcodes "google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	yggstatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
)

func TestToRPCErr(t *testing.T) {
	tests := []struct {
		name    string
		errIn   error
		wantOut error
	}{
		{
			name:    "grpc status",
			errIn:   gstatus.Error(gcodes.Unavailable, "transport is closing"),
			wantOut: xerror.New(code.Code_UNAVAILABLE, "transport is closing"),
		},
		{
			name:    "unexpected eof",
			errIn:   io.ErrUnexpectedEOF,
			wantOut: xerror.New(code.Code_INTERNAL, io.ErrUnexpectedEOF.Error()),
		},
		{
			name:    "context canceled",
			errIn:   context.Canceled,
			wantOut: xerror.New(code.Code_CANCELLED, context.Canceled.Error()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRPCErr(tt.errIn)
			requireStatusErrorEqual(t, got, tt.wantOut)
		})
	}
}

func requireStatusErrorEqual(t *testing.T, got error, want error) {
	t.Helper()

	gotStatus, ok := yggstatus.CoverError(got)
	require.True(t, ok, "got error does not wrap a status: %v", got)

	wantStatus, ok := yggstatus.CoverError(want)
	require.True(t, ok, "want error does not wrap a status: %v", want)

	require.True(t, proto.Equal(gotStatus.Status(), wantStatus.Status()))
}
