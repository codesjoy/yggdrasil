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

package yggdrasil

import (
	"testing"

	"github.com/codesjoy/yggdrasil/v2/config"
	_ "github.com/codesjoy/yggdrasil/v2/remote/credentials/tls"
	"github.com/codesjoy/yggdrasil/v2/server"
)

func TestValidateStartup_Strict_FailsOnMissingTracerBuilder(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.tracer", "missing-tracer")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.tracer", "")
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_NonStrict_WarnsOnly(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.enable", true)
	_ = config.Set("yggdrasil.startup.validate.strict", false)
	_ = config.Set("yggdrasil.tracer", "missing-tracer")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.enable", false)
		_ = config.Set("yggdrasil.tracer", "")
	})

	if err := validateStartup(nil); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidateStartup_Strict_FailsOnMissingRestMarshalerBuilder(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.rest.enable", true)
	_ = config.Set("yggdrasil.rest.marshaler.support", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.rest.enable", false)
		_ = config.Set("yggdrasil.rest.marshaler.support", []string{"jsonpb"})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_Global(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.client.interceptor.unary", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.client.interceptor.unary", []string{})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_ByAppName(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.client.user.interceptor.unary", []string{"nope"})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.client.user.interceptor.unary", []string{})
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsWhenRPCServiceHasNoProtocol(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.server.protocol", []string{})

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.server.protocol", []string{})
	})

	err := validateStartup(&options{
		serviceDesc: map[*server.ServiceDesc]interface{}{
			{ServiceName: "test.service"}: struct{}{},
		},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsWhenRESTHandlersRegisteredButDisabled(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.rest.enable", false)

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.rest.enable", false)
	})

	err := validateStartup(&options{
		serviceDesc: map[*server.ServiceDesc]interface{}{},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{
			{}: {ss: struct{}{}},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingRemoteCredentialsBuilder(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.remote.protocol.grpc.creds_proto", "missing")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.remote.protocol.grpc.creds_proto", "")
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnInvalidTLSCredentialsConfig(t *testing.T) {
	_ = config.Set("yggdrasil.startup.validate.strict", true)
	_ = config.Set("yggdrasil.remote.protocol.grpc.creds_proto", "tls")
	_ = config.Set("yggdrasil.remote.credentials.tls.server.cert_file", "/tmp/missing-cert.pem")
	_ = config.Set("yggdrasil.remote.credentials.tls.server.key_file", "/tmp/missing-key.pem")

	t.Cleanup(func() {
		_ = config.Set("yggdrasil.startup.validate.strict", false)
		_ = config.Set("yggdrasil.remote.protocol.grpc.creds_proto", "")
		_ = config.Set("yggdrasil.remote.credentials.tls.server.cert_file", "")
		_ = config.Set("yggdrasil.remote.credentials.tls.server.key_file", "")
	})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}
