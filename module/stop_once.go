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

package module

import (
	"context"
	"fmt"
	"sync"
)

// StopOnce guards one stop path and returns the first result for all later calls.
type StopOnce struct {
	once sync.Once
	err  error
}

// Do executes fn at most once. Panics are converted into errors and cached.
func (s *StopOnce) Do(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	s.once.Do(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.err = fmt.Errorf("stop panic recovered: %v", recovered)
			}
		}()
		s.err = fn(ctx)
	})
	return s.err
}
