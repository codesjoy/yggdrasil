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

package polaris

import (
	"testing"

	polariscfg "github.com/polarismesh/polaris-go/pkg/config"
)

func TestSDKHolder_SameKey(t *testing.T) {
	h1 := getSDKHolder("same-key", []string{"b", "a"}, nil)
	h2 := getSDKHolder("same-key", []string{"a", "b"}, nil)
	if h1 != h2 {
		t.Fatalf("holders are different for same sdk name")
	}
}

func TestSDKHolder_DifferentName(t *testing.T) {
	h1 := getSDKHolder("sdk-a", []string{"127.0.0.1:1"}, nil)
	h2 := getSDKHolder("sdk-b", []string{"127.0.0.1:1"}, nil)
	if h1 == h2 {
		t.Fatalf("holders are same for different sdk names")
	}
}

func TestSDKHolder_DifferentAddresses(t *testing.T) {
	h1 := getSDKHolder("same-sdk", []string{"127.0.0.1:1"}, nil)
	h2 := getSDKHolder("same-sdk", []string{"127.0.0.1:2"}, nil)
	if h1 == h2 {
		t.Fatalf("holders are same for different addresses")
	}
}

func TestSDKHolder_DifferentConfigAddresses(t *testing.T) {
	h1 := getSDKHolder("same-sdk-config", nil, []string{"127.0.0.1:3"})
	h2 := getSDKHolder("same-sdk-config", nil, []string{"127.0.0.1:4"})
	if h1 == h2 {
		t.Fatalf("holders are same for different config addresses")
	}
}

func TestApplyAccessTokenToConfig_SetsTokens(t *testing.T) {
	c := polariscfg.NewDefaultConfiguration([]string{"127.0.0.1:1"})
	applyTokenToConfig(c, "token")
	if got := c.GetGlobal().GetServerConnector().GetToken(); got != "token" {
		t.Fatalf("global.serverConnector.token = %q, want %q", got, "token")
	}
	if got := c.GetConfigFile().GetConfigConnectorConfig().GetToken(); got != "token" {
		t.Fatalf("config.configConnector.token = %q, want %q", got, "token")
	}
}
