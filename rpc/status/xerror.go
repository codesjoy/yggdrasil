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

package status

import (
	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
)

func fromXError(err error) (*Status, bool) {
	errorCode, ok := xerror.CodeOf(err)
	if !ok {
		return nil, false
	}

	st := WithCode(errorCode, err)
	reason, domain, metadata, ok := xerror.ReasonOf(err)
	if ok {
		_ = st.WithDetails(&errdetails.ErrorInfo{
			Reason:   reason,
			Domain:   domain,
			Metadata: metadata,
		})
	}
	return st, true
}
