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
	"go.opentelemetry.io/otel/metric"
)

// MeterProviderBuilder is a function that returns a MeterProvider.
type MeterProviderBuilder func(name string) metric.MeterProvider

var meterBuilders = make(map[string]MeterProviderBuilder)

// RegisterMeterProviderBuilder registers a MeterProviderBuilder.
func RegisterMeterProviderBuilder(name string, constructor MeterProviderBuilder) {
	meterBuilders[name] = constructor
}

// GetMeterProviderBuilder returns a MeterProviderBuilder.
func GetMeterProviderBuilder(name string) (MeterProviderBuilder, bool) {
	constructor, ok := meterBuilders[name]
	return constructor, ok
}
