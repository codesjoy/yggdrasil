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

package convert

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertTimestamp(t *testing.T) {
	specs := []struct {
		name    string
		input   string
		output  *timestamppb.Timestamp
		wanterr bool
	}{
		{
			name:  "a valid RFC3339 timestamp",
			input: `"2016-05-10T10:19:13.123Z"`,
			output: &timestamppb.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			wanterr: false,
		},
		{
			name:  "a valid RFC3339 timestamp without double quotation",
			input: "2016-05-10T10:19:13.123Z",
			output: &timestamppb.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			wanterr: false,
		},
		{
			name:    "invalid timestamp",
			input:   `"05-10-2016T10:19:13.123Z"`,
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON number",
			input:   "123",
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON bool",
			input:   "true",
			output:  nil,
			wanterr: true,
		},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			ts, err := Timestamp(spec.input)
			switch {
			case err != nil && !spec.wanterr:
				t.Errorf("got unexpected error\n%#v", err)
			case err == nil && spec.wanterr:
				t.Errorf("did not error when expecte")
			case !proto.Equal(ts, spec.output):
				t.Errorf(
					"when testing %s; got\n%#v\nexpected\n%#v",
					spec.name,
					ts,
					spec.output,
				)
			}
		})
	}
}

func TestConvertDuration(t *testing.T) {
	specs := []struct {
		name    string
		input   string
		output  *durationpb.Duration
		wanterr bool
	}{
		{
			name:  "a valid duration",
			input: `"123.456s"`,
			output: &durationpb.Duration{
				Seconds: 123,
				Nanos:   456000000,
			},
			wanterr: false,
		},
		{
			name:  "a valid duration without double quotation",
			input: "123.456s",
			output: &durationpb.Duration{
				Seconds: 123,
				Nanos:   456000000,
			},
			wanterr: false,
		},
		{
			name:    "invalid duration",
			input:   `"123years"`,
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON number",
			input:   "123",
			output:  nil,
			wanterr: true,
		},
		{
			name:    "JSON bool",
			input:   "true",
			output:  nil,
			wanterr: true,
		},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			ts, err := Duration(spec.input)
			switch {
			case err != nil && !spec.wanterr:
				t.Errorf("got unexpected error\n%#v", err)
			case err == nil && spec.wanterr:
				t.Errorf("did not error when expecte")
			case !proto.Equal(ts, spec.output):
				t.Errorf(
					"when testing %s; got\n%#v\nexpected\n%#v",
					spec.name,
					ts,
					spec.output,
				)
			}
		})
	}
}
