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

package rest

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEntry_Write_LevelSelection(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "200 OK", status: 200},
		{name: "404 Not Found", status: 404},
		{name: "500 Internal Server Error", status: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test", nil)
			entry := &logEntry{r: r}
			assert.NotPanics(t, func() {
				entry.Write(tt.status, 0, nil, 10*time.Millisecond, nil)
			})
		})
	}
}

func TestLogEntry_Panic(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	entry := &logEntry{r: r}
	assert.NotPanics(t, func() {
		entry.Panic("something went wrong", []byte("stack trace here"))
	})
}

func TestLogFormatter_NewLogEntry(t *testing.T) {
	formatter := &logFormatter{}
	r := httptest.NewRequest("POST", "/api/test", nil)
	entry := formatter.NewLogEntry(r)
	require.NotNil(t, entry)

	le, ok := entry.(*logEntry)
	require.True(t, ok)
	assert.Equal(t, r, le.r)
}
