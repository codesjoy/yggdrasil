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

package otel

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

// ParseAttributes parse attributes map to []attribute.KeyValue
func ParseAttributes(attrsMap map[string]interface{}) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	for k, v := range attrsMap {
		switch v := v.(type) {
		case bool:
			attrs = append(attrs, attribute.Bool(k, v))
		case string:
			attrs = append(attrs, attribute.String(k, v))
		case int64:
			attrs = append(attrs, attribute.Int64(k, v))
		case float64:
			attrs = append(attrs, attribute.Float64(k, v))
		case []float64:
			attrs = append(attrs, attribute.Float64Slice(k, v))
		case []int64:
			attrs = append(attrs, attribute.Int64Slice(k, v))
		case []string:
			attrs = append(attrs, attribute.StringSlice(k, v))
		case []bool:
			attrs = append(attrs, attribute.BoolSlice(k, v))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
		}
	}
	return attrs
}
