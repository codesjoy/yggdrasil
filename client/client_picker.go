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

package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/codesjoy/yggdrasil/v2/balancer"
	istatus "github.com/codesjoy/yggdrasil/v2/internal/status"
	"github.com/codesjoy/yggdrasil/v2/remote"
	"github.com/codesjoy/yggdrasil/v2/status"
	"google.golang.org/genproto/googleapis/rpc/code"
)

// pickerSnap store  a picker and a channel used to signal that a picker
// newer than this one is available.
type pickerSnap struct {
	// picker is the picker produced by the LB policy.  May be nil if a picker
	// has never been produced.
	picker balancer.Picker
	// blockingCh is closed when the picker has been invalidated because there
	// is a new one available.
	blockingCh chan struct{}
}

// updatePicker is called by UpdateState calls from the LB policy. It
// unblocks all blocked pick.
func (c *client) updatePicker(p balancer.Picker) {
	old := c.pickerSnap.Swap(&pickerSnap{
		picker:     p,
		blockingCh: make(chan struct{}),
	})
	close(old.blockingCh)
}

func (c *client) pick(failFast bool, info *balancer.RPCInfo) (balancer.PickResult, error) {
	var ch chan struct{}

	var lastPickErr error

	for {
		pg := c.pickerSnap.Load()
		if pg == nil {
			return nil, ErrClientClosing
		}
		if pg.picker == nil {
			ch = pg.blockingCh
		}
		if ch == pg.blockingCh {
			// This could happen when either:
			// - cp.picker is nil (the previous if condition), or
			// - we have already called pick on the current picker.
			select {
			case <-info.Ctx.Done():
				var errStr string
				if lastPickErr != nil {
					errStr = "latest balancer error: " + lastPickErr.Error()
				} else {
					errStr = fmt.Sprintf("%v while waiting for connections to become ready", info.Ctx.Err())
				}
				switch err := info.Ctx.Err(); {
				case errors.Is(err, context.DeadlineExceeded):
					return nil, status.New(code.Code_DEADLINE_EXCEEDED, errStr)
				case errors.Is(err, context.Canceled):
					return nil, status.New(code.Code_CANCELLED, errStr)
				}
			case <-ch:
			}
			continue
		}

		//nolint:staticcheck // SA4006: ch is used in the next iteration of the loop
		ch = pg.blockingCh
		p := pg.picker

		pickResult, err := p.Next(*info)
		if err != nil {
			if errors.Is(err, balancer.ErrNoAvailableInstance) {
				ch = pg.blockingCh
				continue
			}
			if st, ok := status.CoverError(err); ok {
				if istatus.IsRestrictedControlPlaneCode(st) {
					err = status.New(
						code.Code_INTERNAL,
						fmt.Sprintf("received picker error with illegal status: %v", err),
					)
				}
				return nil, err
			}
			// For all other errors, wait for ready RPCs should block and other
			// RPCs should fail with unavailable.
			if !failFast {
				lastPickErr = err
				ch = pg.blockingCh
				continue
			}
			return nil, status.New(code.Code_UNAVAILABLE, err.Error())
		}

		if pickResult.RemoteClient().State() != remote.Ready {
			ch = pg.blockingCh
			continue
		}
		return pickResult, nil
	}
}
