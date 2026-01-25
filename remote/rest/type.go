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
	"net/http"
)

// A HandlerFunc handles a specific pair of path pattern and HTTP method.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// ServerInfo provides information about the rest server.
type ServerInfo interface {
	GetAddress() string
	GetAttributes() map[string]string
}

// Server is the interface that wraps the Serve method.
type Server interface {
	RPCHandle(method, path string, f HandlerFunc)
	RawHandle(method, path string, h http.HandlerFunc)
	Start() error
	Serve() error
	Stop() error
	Info() ServerInfo
}
