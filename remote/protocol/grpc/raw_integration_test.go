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

package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/remote"
	jsonrawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/jsonraw"
	rawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/raw"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	yggstatus "github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

const (
	rawUnaryMethod        = "/raw.Test/Unary"
	rawServerStreamMethod = "/raw.Test/ServerStream"
	rawClientStreamMethod = "/raw.Test/ClientStream"
	rawBidiStreamMethod   = "/raw.Test/Bidi"

	jsonRawUnaryMethod      = "/jsonraw.Test/Unary"
	jsonRawClientStreamMode = "/jsonraw.Test/ClientStream"
	jsonRawBidiStreamMethod = "/jsonraw.Test/Bidi"
)

func TestRawUnaryCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", rawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
		&stream.Desc{},
		rawUnaryMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte("ping")))

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, []byte("unary:ping"), reply)
}

func TestRawServerStreamCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", rawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
		&stream.Desc{ServerStreams: true},
		rawServerStreamMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte("ping")))

	var messages [][]byte
	for {
		var reply []byte
		err = cs.RecvMsg(&reply)
		if isSuccessfulStreamEnd(err) {
			break
		}
		require.NoError(t, err)
		messages = append(messages, append([]byte(nil), reply...))
	}

	assert.Equal(t, [][]byte{
		[]byte("stream-1:ping"),
		[]byte("stream-2:ping"),
	}, messages)
}

func TestRawClientStreamCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", rawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
		&stream.Desc{ClientStreams: true},
		rawClientStreamMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte("a")))
	require.NoError(t, cs.SendMsg([]byte("b")))
	require.NoError(t, cs.CloseSend())

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, []byte("a|b"), reply)
}

func TestRawBidiStreamCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", rawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
		&stream.Desc{ClientStreams: true, ServerStreams: true},
		rawBidiStreamMethod,
	)
	require.NoError(t, err)

	require.NoError(t, cs.SendMsg([]byte("a")))
	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, []byte("echo:a"), reply)

	require.NoError(t, cs.SendMsg([]byte("b")))
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, []byte("echo:b"), reply)

	require.NoError(t, cs.CloseSend())
	assert.True(t, isSuccessfulStreamEnd(cs.RecvMsg(&reply)))
}

func TestRawUnaryCallWithGzip(t *testing.T) {
	cc := newRawTestClientConn(t, "gzip", rawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(rawenc.Name)),
		&stream.Desc{},
		rawUnaryMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte("ping")))

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.Equal(t, []byte("unary:ping"), reply)
}

func TestJSONRawUnaryCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", jsonRawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
		&stream.Desc{},
		jsonRawUnaryMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte(`{"message":"ping"}`)))

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.JSONEq(t, `{"message":"pong"}`, string(reply))
}

func TestJSONRawClientStreamCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", jsonRawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
		&stream.Desc{ClientStreams: true},
		jsonRawClientStreamMode,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte(`{"message":"a"}`)))
	require.NoError(t, cs.SendMsg([]byte(`{"message":"b"}`)))
	require.NoError(t, cs.CloseSend())

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.JSONEq(t, `{"messages":["a","b"]}`, string(reply))
}

func TestJSONRawBidiStreamCall(t *testing.T) {
	cc := newRawTestClientConn(t, "", jsonRawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
		&stream.Desc{ClientStreams: true, ServerStreams: true},
		jsonRawBidiStreamMethod,
	)
	require.NoError(t, err)

	require.NoError(t, cs.SendMsg([]byte(`{"message":"a"}`)))
	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.JSONEq(t, `{"message":"echo:a"}`, string(reply))

	require.NoError(t, cs.SendMsg([]byte(`{"message":"b"}`)))
	require.NoError(t, cs.RecvMsg(&reply))
	assert.JSONEq(t, `{"message":"echo:b"}`, string(reply))

	require.NoError(t, cs.CloseSend())
	assert.True(t, isSuccessfulStreamEnd(cs.RecvMsg(&reply)))
}

func TestJSONRawUnaryCallWithGzip(t *testing.T) {
	cc := newRawTestClientConn(t, "gzip", jsonRawTestMethodHandle)

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
		&stream.Desc{},
		jsonRawUnaryMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte(`{"message":"ping"}`)))

	var reply []byte
	require.NoError(t, cs.RecvMsg(&reply))
	assert.JSONEq(t, `{"message":"pong"}`, string(reply))
}

func newRawTestClientConn(
	t *testing.T,
	compressor string,
	handle remote.MethodHandle,
) *clientConn {
	t.Helper()

	serviceName := sanitizeServiceName(t.Name())
	configPrefix := fmt.Sprintf("{%s}", serviceName)
	setConfigValue(t, config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp")
	setConfigValue(
		t,
		config.Join(config.KeyBase, "remote", "protocol", scheme, "address"),
		"127.0.0.1:0",
	)
	setConfigValue(t, config.Join(config.KeyBase, "remote", "protocol", scheme, "codeProto"), "")
	setConfigValue(
		t,
		config.Join(config.KeyBase, "client", configPrefix, "protocol_config", scheme, "network"),
		"tcp",
	)
	setConfigValue(
		t,
		config.Join(
			config.KeyBase,
			"client",
			configPrefix,
			"protocol_config",
			scheme,
			"compressor",
		),
		compressor,
	)

	srv, err := newServer(handle)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Handle()
	}()

	endpoint := resolver.BaseEndpoint{
		Address:  srv.Info().Address,
		Protocol: scheme,
	}
	cli, err := newClient(
		context.Background(),
		serviceName,
		endpoint,
		stats.GetClientHandler(),
		func(remote.ClientState) {},
	)
	require.NoError(t, err)
	cc := cli.(*clientConn)
	cc.Connect()
	require.Eventually(t, func() bool {
		return cc.State() == remote.Ready
	}, 5*time.Second, 20*time.Millisecond)

	t.Cleanup(func() {
		_ = cc.Close()
		require.NoError(t, srv.Stop(context.Background()))
		select {
		case err := <-serveErrCh:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("grpc server did not stop in time")
		}
	})

	return cc
}

func setConfigValue(t *testing.T, key string, value any) {
	t.Helper()
	require.NoError(t, config.Set(key, value))
}

func sanitizeServiceName(name string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", "=", "-", "(", "-", ")", "-")
	return replacer.Replace(strings.ToLower(name))
}

func isSuccessfulStreamEnd(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	st, ok := yggstatus.CoverError(err)
	return ok && st.Code() == code.Code_OK
}

func rawTestMethodHandle(ss remote.ServerStream) {
	var (
		reply any
		err   error
	)
	defer func() {
		ss.Finish(reply, err)
	}()

	switch ss.Method() {
	case rawUnaryMethod:
		err = ss.Start(false, false)
		if err != nil {
			return
		}
		var req []byte
		if err = ss.RecvMsg(&req); err != nil {
			return
		}
		reply = append([]byte("unary:"), req...)
	case rawServerStreamMethod:
		err = ss.Start(false, true)
		if err != nil {
			return
		}
		var req []byte
		if err = ss.RecvMsg(&req); err != nil {
			return
		}
		for _, item := range [][]byte{
			append([]byte("stream-1:"), req...),
			append([]byte("stream-2:"), req...),
		} {
			if err = ss.SendMsg(item); err != nil {
				return
			}
		}
		for {
			var extra []byte
			err = ss.RecvMsg(&extra)
			if err == io.EOF {
				err = nil
				return
			}
			if err != nil {
				return
			}
		}
	case rawClientStreamMethod:
		err = ss.Start(true, false)
		if err != nil {
			return
		}
		var parts [][]byte
		for {
			var req []byte
			err = ss.RecvMsg(&req)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				return
			}
			parts = append(parts, append([]byte(nil), req...))
		}
		err = ss.SendMsg(bytes.Join(parts, []byte("|")))
	case rawBidiStreamMethod:
		err = ss.Start(true, true)
		if err != nil {
			return
		}
		for {
			var req []byte
			err = ss.RecvMsg(&req)
			if err == io.EOF {
				err = nil
				return
			}
			if err != nil {
				return
			}
			if err = ss.SendMsg(append([]byte("echo:"), req...)); err != nil {
				return
			}
		}
	default:
		err = xerror.New(code.Code_UNIMPLEMENTED, "unknown method")
	}
}

func jsonRawTestMethodHandle(ss remote.ServerStream) {
	var (
		reply any
		err   error
	)
	defer func() {
		ss.Finish(reply, err)
	}()

	switch ss.Method() {
	case jsonRawUnaryMethod:
		err = ss.Start(false, false)
		if err != nil {
			return
		}
		var req []byte
		if err = ss.RecvMsg(&req); err != nil {
			return
		}
		reply = []byte(`{"message":"pong"}`)
	case jsonRawClientStreamMode:
		err = ss.Start(true, false)
		if err != nil {
			return
		}
		var messages []string
		for {
			var req []byte
			err = ss.RecvMsg(&req)
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				return
			}
			messages = append(messages, extractJSONMessage(req))
		}
		err = ss.SendMsg([]byte(fmt.Sprintf(`{"messages":["%s"]}`, strings.Join(messages, `","`))))
	case jsonRawBidiStreamMethod:
		err = ss.Start(true, true)
		if err != nil {
			return
		}
		for {
			var req []byte
			err = ss.RecvMsg(&req)
			if err == io.EOF {
				err = nil
				return
			}
			if err != nil {
				return
			}
			msg := extractJSONMessage(req)
			if err = ss.SendMsg([]byte(fmt.Sprintf(`{"message":"echo:%s"}`, msg))); err != nil {
				return
			}
		}
	default:
		err = xerror.New(code.Code_UNIMPLEMENTED, "unknown method")
	}
}

func extractJSONMessage(b []byte) string {
	s := string(b)
	prefix := `{"message":"`
	suffix := `"}`
	if strings.HasPrefix(s, prefix) && strings.HasSuffix(s, suffix) {
		return strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
	}
	return s
}
