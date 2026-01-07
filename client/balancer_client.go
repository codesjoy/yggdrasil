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

// Package client provides balancer client
package client

import (
	"context"
	"log/slog"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/utils/xsync"
)

type balancerClient struct {
	cli        *client
	serializer *xsync.CallbackSerializer
}

// UpdateState updates the state of the client
func (bc *balancerClient) UpdateState(state balancer.State) {
	bc.cli.updatePicker(state.Picker)
}

// NewRemoteClient creates a new remote client using centralized ConnManager
func (bc *balancerClient) NewRemoteClient(
	endpoint resolver.Endpoint,
	ops balancer.NewRemoteClientOptions,
) (remote.Client, error) {
	rc, err := bc.cli.remoteClientManager.GetOrCreate(
		endpoint,
		bc.createStateListener(ops.StateListener),
	)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

func (bc *balancerClient) createStateListener(f func(remote.ClientState)) func(remote.ClientState) {
	return func(state remote.ClientState) {
		bc.serializer.ScheduleOr(func(_ context.Context) {
			f(state)
		}, func() {
			slog.Error("createStateListener failed")
		})
	}
}
