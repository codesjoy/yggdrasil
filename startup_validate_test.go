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

	"github.com/codesjoy/yggdrasil/v2/internal/configtest"
	_ "github.com/codesjoy/yggdrasil/v2/remote/credentials/tls"
	"github.com/codesjoy/yggdrasil/v2/server"
)

func TestValidateStartup_Strict_FailsOnMissingTracerBuilder(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_NonStrict_WarnsOnly(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.enable", true)
	configtest.Set(t, "yggdrasil.admin.validation.strict", false)
	configtest.Set(t, "yggdrasil.telemetry.tracer", "missing-tracer")

	if err := validateStartup(nil); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestValidateStartup_Strict_FailsOnMissingRestMarshalerBuilder(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.transports.http.rest.port", 0)
	configtest.Set(t, "yggdrasil.transports.http.rest.marshaler.support", []string{"nope"})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_Global(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.clients.defaults.interceptors.unary", []string{"nope"})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingClientInterceptor_ByAppName(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.clients.services.user.interceptors.unary", []string{"nope"})

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsWhenRPCServiceHasNoProtocol(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.server.transports", []string{})

	err := validateStartup(&options{
		serviceDesc: map[*server.ServiceDesc]interface{}{
			&server.ServiceDesc{ServiceName: "test.service"}: struct{}{},
		},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsWhenRESTHandlersRegisteredButDisabled(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)

	err := validateStartup(&options{
		serviceDesc: map[*server.ServiceDesc]interface{}{},
		restServiceDesc: map[*server.RestServiceDesc]restServiceDesc{
			&server.RestServiceDesc{}: {ss: struct{}{}},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnMissingRemoteCredentialsBuilder(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.transports.grpc.server.creds_proto", "missing")

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateStartup_Strict_FailsOnInvalidTLSCredentialsConfig(t *testing.T) {
	configtest.Set(t, "yggdrasil.admin.validation.strict", true)
	configtest.Set(t, "yggdrasil.transports.grpc.server.creds_proto", "tls")
	configtest.Set(t, "yggdrasil.transports.grpc.credentials.tls.server.cert_file", "/tmp/missing-cert.pem")
	configtest.Set(t, "yggdrasil.transports.grpc.credentials.tls.server.key_file", "/tmp/missing-key.pem")

	if err := validateStartup(nil); err == nil {
		t.Fatalf("expected error")
	}
}
