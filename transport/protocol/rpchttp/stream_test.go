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

package rpchttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"

	"github.com/codesjoy/pkg/basic/xerror"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	transportmeta "github.com/codesjoy/yggdrasil/v3/transport/internal/httpmeta"
	"github.com/codesjoy/yggdrasil/v3/transport/protocol/grpc/encoding/proto/codec_perf"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type countingStatsHandler struct {
	begins      atomic.Int32
	ends        atomic.Int32
	inPayloads  atomic.Int32
	outPayloads atomic.Int32
}

func (h *countingStatsHandler) TagRPC(ctx context.Context, _ stats.RPCTagInfo) context.Context {
	return ctx
}

func (h *countingStatsHandler) HandleRPC(_ context.Context, item stats.RPCStats) {
	switch item.(type) {
	case *stats.RPCBeginBase:
		h.begins.Add(1)
	case *stats.RPCEndBase:
		h.ends.Add(1)
	case *stats.RPCInPayloadBase:
		h.inPayloads.Add(1)
	case *stats.RPCOutPayloadBase:
		h.outPayloads.Add(1)
	}
}

func (*countingStatsHandler) TagChannel(ctx context.Context, _ stats.ChanTagInfo) context.Context {
	return ctx
}

func (*countingStatsHandler) HandleChannel(context.Context, stats.ChanStats) {}

func newTestServerStream(req *http.Request, w http.ResponseWriter) *httpServerStream {
	return &httpServerStream{
		ctx:                context.Background(),
		method:             "/test.Method",
		req:                req,
		w:                  w,
		statsHandler:       stats.NoOpHandler,
		configuredInbound:  marshaler.NewJSONPbMarshalerWithConfig(nil),
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
}

func TestHTTPServerStream_Method(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", nil)
	ss := newTestServerStream(req, httptest.NewRecorder())
	assert.Equal(t, "/test.Method", ss.Method())
}

func TestHTTPServerStream_Context(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest(http.MethodPost, "/test.Method", nil)
	ss := &httpServerStream{
		ctx:                ctx,
		method:             "/test.Method",
		req:                req,
		w:                  httptest.NewRecorder(),
		statsHandler:       stats.NoOpHandler,
		configuredInbound:  marshaler.NewJSONPbMarshalerWithConfig(nil),
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
	assert.Equal(t, ctx, ss.Context())
}

func TestHTTPServerStream_SetHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", nil)
	ss := newTestServerStream(req, httptest.NewRecorder())

	err := ss.SetHeader(metadata.Pairs("key1", "val1"))
	require.NoError(t, err)

	err = ss.SetHeader(metadata.Pairs("key2", "val2"))
	require.NoError(t, err)

	// Both headers should be accumulated.
	require.NotNil(t, ss.headerMD)
	assert.Equal(t, []string{"val1"}, ss.headerMD.Get("key1"))
	assert.Equal(t, []string{"val2"}, ss.headerMD.Get("key2"))
}

func TestHTTPServerStream_SendHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", nil)
	ss := newTestServerStream(req, httptest.NewRecorder())

	// SendHeader delegates to SetHeader.
	err := ss.SendHeader(metadata.Pairs("key1", "val1"))
	require.NoError(t, err)
	assert.Equal(t, []string{"val1"}, ss.headerMD.Get("key1"))
}

func TestHTTPServerStream_SetTrailer(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", nil)
	ss := newTestServerStream(req, httptest.NewRecorder())

	ss.SetTrailer(metadata.Pairs("tkey1", "tval1"))
	ss.SetTrailer(metadata.Pairs("tkey2", "tval2"))

	require.NotNil(t, ss.trailerMD)
	assert.Equal(t, []string{"tval1"}, ss.trailerMD.Get("tkey1"))
	assert.Equal(t, []string{"tval2"}, ss.trailerMD.Get("tkey2"))
}

func TestHTTPServerStream_Start_StreamingRejection(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString("{}"))
	ss := newTestServerStream(req, httptest.NewRecorder())

	err := ss.Start(true, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support streaming")

	// Also test server streaming rejection.
	err = ss.Start(false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support streaming")
}

func TestHTTPServerStream_Start_DuplicateStart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString("{}"))
	ss := newTestServerStream(req, httptest.NewRecorder())

	err := ss.Start(false, false)
	require.NoError(t, err)

	// Second start should fail.
	err = ss.Start(false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream already started")
}

func TestHTTPServerStream_RecvMsg(t *testing.T) {
	t.Run("unmarshals body on first call", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/test.Method",
			bytes.NewBufferString(`{"name":"test"}`),
		)
		w := httptest.NewRecorder()
		ss := newTestServerStream(req, w)

		err := ss.Start(false, false)
		require.NoError(t, err)

		var msg struct {
			Name string `json:"name"`
		}
		err = ss.RecvMsg(&msg)
		require.NoError(t, err)
		assert.Equal(t, "test", msg.Name)
	})

	t.Run("returns io.EOF on second call", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
		w := httptest.NewRecorder()
		ss := newTestServerStream(req, w)

		err := ss.Start(false, false)
		require.NoError(t, err)

		var msg struct{}
		err = ss.RecvMsg(&msg)
		require.NoError(t, err)

		err = ss.RecvMsg(&msg)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("empty body returns nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBuffer(nil))
		w := httptest.NewRecorder()
		ss := newTestServerStream(req, w)

		err := ss.Start(false, false)
		require.NoError(t, err)

		var msg struct{}
		err = ss.RecvMsg(&msg)
		require.NoError(t, err)
	})
}

func TestHTTPServerStream_Finish_Error(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	rpcErr := xerror.New(code.Code_NOT_FOUND, "item not found")
	ss.Finish(nil, rpcErr)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "item not found")
}

func TestHTTPServerStream_Finish_NilReply(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	ss.Finish(nil, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPServerStream_Finish_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	reply := &testMessage{Value: "hello"}
	ss.Finish(reply, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello")
}

func TestHTTPServerStream_Finish_MarshalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	// Replace outbound with a failing marshaler.
	ss.outbound = &failingMarshaler{}

	ss.Finish(&testMessage{Value: "data"}, nil)
	// Should get 500 fallback.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHTTPServerStream_Finish_DuplicateFinish(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	ss.Finish(nil, nil)
	// Second call should be a no-op (no panic, no double-write).
	ss.Finish(nil, errors.New("should be ignored"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPServerStream_SendMsg(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	// SendMsg delegates to Finish.
	err = ss.SendMsg(&testMessage{Value: "sent"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sent")
}

func TestHTTPServerStream_Finish_WithTrailers(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	req.Header.Set("TE", "trailers")
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	ss.SetTrailer(metadata.Pairs("extra-info", "details"))
	ss.Finish(nil, nil)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPServerStream_Finish_ErrorWithTrailers(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	req.Header.Set("TE", "trailers")
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	ss.SetTrailer(metadata.Pairs("err-trailer", "value"))
	rpcErr := xerror.New(code.Code_INVALID_ARGUMENT, "bad request")
	ss.Finish(nil, rpcErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTPServerStream_Finish_NonProtoReply(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	// Non-proto.Message reply should still work (falls through to raw write).
	ss.Finish("plain string", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPServerStream_Finish_ProtoReply(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test.Method", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	ss := newTestServerStream(req, w)

	err := ss.Start(false, false)
	require.NoError(t, err)

	// codec_perf.Buffer is a real proto.Message -- tests the proto path in Finish.
	reply := &codec_perf.Buffer{Body: []byte("proto data")}
	ss.Finish(reply, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	// The body field is []byte which is base64-encoded in JSON proto serialization.
	assert.Contains(t, w.Body.String(), "body")
	// Content-Type should be set to application/json for a JSONPb marshaler.
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestHTTPClientStream_CloseSend(t *testing.T) {
	cs := &httpClientStream{
		ctx: context.Background(),
	}
	err := cs.CloseSend()
	require.NoError(t, err)
}

func TestHTTPClientStream_Context(t *testing.T) {
	ctx := context.Background()
	cs := &httpClientStream{
		ctx: ctx,
	}
	assert.Equal(t, ctx, cs.Context())
}

func TestHTTPClientStream_SendMsg_DoubleSend(t *testing.T) {
	cs := &httpClientStream{
		ctx:                context.Background(),
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
	err := cs.SendMsg(&testMessage{Value: "first"})
	require.NoError(t, err)

	// Second SendMsg should fail.
	err = cs.SendMsg(&testMessage{Value: "second"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already sent")
}

func TestHTTPClientStream_RecvMsg_WithoutSend(t *testing.T) {
	cs := &httpClientStream{
		ctx:                context.Background(),
		configuredInbound:  marshaler.NewJSONPbMarshalerWithConfig(nil),
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
	var msg testMessage
	err := cs.RecvMsg(&msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request not sent")
}

func TestHTTPClientStream_Trailer_Empty(t *testing.T) {
	cs := &httpClientStream{
		ctx: context.Background(),
	}
	tr := cs.Trailer()
	assert.Equal(t, metadata.MD{}, tr)
}

func TestHTTPClientStream_Header_AlreadyReceived(t *testing.T) {
	cs := &httpClientStream{
		ctx:      context.Background(),
		reqSent:  true,
		respRecv: true,
		header:   metadata.Pairs("key1", "val1"),
	}
	h, err := cs.Header()
	require.NoError(t, err)
	assert.Equal(t, []string{"val1"}, h.Get("key1"))
}

func TestHTTPClientStream_Header_AlreadyReceivedWithErr(t *testing.T) {
	testErr := errors.New("connection refused")
	cs := &httpClientStream{
		ctx:      context.Background(),
		reqSent:  true,
		respRecv: true,
		header:   metadata.Pairs("key1", "val1"),
		respErr:  testErr,
	}
	h, err := cs.Header()
	assert.Equal(t, testErr, err)
	assert.Equal(t, []string{"val1"}, h.Get("key1"))
}

func TestHTTPClientStream_Header_WaitForResponse(t *testing.T) {
	cs := &httpClientStream{
		ctx:             context.Background(),
		reqSent:         true,
		responseStarted: true,
		headersReady:    make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		h, err := cs.Header()
		require.NoError(t, err)
		assert.Equal(t, []string{"val1"}, h.Get("key1"))
		close(done)
	}()

	// Simulate the response arriving.
	ch := cs.headersReady
	cs.finishResponse(metadata.Pairs("key1", "val1"), nil, nil, nil, ch)

	<-done
}

func TestHTTPClientStream_Header_WaitNilHeader(t *testing.T) {
	cs := &httpClientStream{
		ctx:             context.Background(),
		reqSent:         true,
		responseStarted: true,
		headersReady:    make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		h, err := cs.Header()
		assert.Equal(t, metadata.MD{}, h)
		assert.NoError(t, err)
		close(done)
	}()

	cs.finishResponse(nil, nil, nil, nil, cs.headersReady)
	<-done
}

func TestHTTPClientStream_finishResponse(t *testing.T) {
	cs := &httpClientStream{
		ctx: context.Background(),
	}
	h := metadata.Pairs("h1", "v1")
	tr := metadata.Pairs("t1", "v1")
	testErr := errors.New("test error")

	ch := make(chan struct{})
	cs.finishResponse(h, tr, nil, testErr, ch)

	assert.True(t, cs.responseReady)
	assert.Equal(t, testErr, cs.respErr)
	assert.Equal(t, []string{"v1"}, cs.header.Get("h1"))
	assert.Equal(t, []string{"v1"}, cs.trailer.Get("t1"))
}

func TestHTTPClientStream_finishResponse_NilChannel(t *testing.T) {
	cs := &httpClientStream{
		ctx: context.Background(),
	}
	// finishResponse with nil channel should not panic.
	cs.finishResponse(nil, nil, nil, nil, nil)
	assert.True(t, cs.responseReady)
}

func TestHTTPClientStream_finishResponse_ChannelAlreadyClosed(t *testing.T) {
	cs := &httpClientStream{
		ctx: context.Background(),
	}
	ch := make(chan struct{})
	close(ch)
	// Should not panic when channel is already closed.
	cs.finishResponse(nil, nil, nil, nil, ch)
	assert.True(t, cs.responseReady)
}

// testMessage is a simple struct for testing marshal/unmarshal.
// It is NOT a proto.Message; the Finish path handles non-proto replies
// via the raw-write fallback path.
type testMessage struct {
	Value string `json:"value,omitempty"`
}

// failingMarshaler always returns an error on Marshal.
type failingMarshaler struct{}

func (f *failingMarshaler) Marshal(v interface{}) ([]byte, error) {
	return nil, errors.New("marshal failed")
}

func (f *failingMarshaler) Unmarshal(data []byte, v interface{}) error {
	return errors.New("unmarshal failed")
}

func (f *failingMarshaler) ContentType(v interface{}) string {
	return "application/json"
}

func (f *failingMarshaler) NewDecoder(r io.Reader) marshaler.Decoder {
	return &failingDecoder{r: r}
}

func (f *failingMarshaler) NewEncoder(w io.Writer) marshaler.Encoder {
	return &failingEncoder{w: w}
}

type (
	failingDecoder struct{ r io.Reader }
	failingEncoder struct{ w io.Writer }
)

func (d *failingDecoder) Decode(v interface{}) error { return errors.New("decode failed") }
func (e *failingEncoder) Encode(v interface{}) error { return errors.New("encode failed") }

// newTestClientStream creates an httpClientStream wired to an httptest.Server.
func newTestClientStream(
	t *testing.T,
	handler http.HandlerFunc,
) (*httpClientStream, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &httpClientStream{
		ctx:                context.Background(),
		method:             "/test.Service/Method",
		endpointAddr:       ts.URL,
		httpClient:         ts.Client(),
		statsHandler:       stats.NoOpHandler,
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
		configuredInbound:  marshaler.NewJSONPbMarshalerWithConfig(nil),
	}, ts
}

func TestHTTPClientStream_SendMsg_FirstCall(t *testing.T) {
	cs := &httpClientStream{
		ctx:                context.Background(),
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
	err := cs.SendMsg(&testMessage{Value: "hello"})
	require.NoError(t, err)
	assert.True(t, cs.reqSent)
	assert.NotNil(t, cs.reqBytes)
	assert.Contains(t, string(cs.reqBytes), "hello")
}

func TestHTTPClientStream_SendMsg_MarshalError(t *testing.T) {
	cs := &httpClientStream{
		ctx:                context.Background(),
		configuredOutbound: &failingMarshaler{},
	}
	err := cs.SendMsg(&testMessage{Value: "hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal failed")
	assert.False(t, cs.reqSent)

	// Retry with a working marshaler.
	cs.configuredOutbound = marshaler.NewJSONPbMarshalerWithConfig(nil)
	err = cs.SendMsg(&testMessage{Value: "hello"})
	require.NoError(t, err)
	assert.True(t, cs.reqSent)
}

func TestHTTPClientStream_RecvMsg_HappyPath(t *testing.T) {
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(MetadataHeaderPrefix+"X-Custom", "custom-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"resp"}`))
	})

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.NoError(t, err)
	assert.Equal(t, "resp", msg.Value)

	h, err := cs.Header()
	require.NoError(t, err)
	assert.Equal(t, []string{"custom-value"}, h.Get("X-Custom"))
}

func TestHTTPClientStream_RecvMsg_Non200_ProtoStatus(t *testing.T) {
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"not found"}`))
	})

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHTTPClientStream_RecvMsg_Non200_RawBody(t *testing.T) {
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error body"))
	})

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "internal server error body")
	assert.Contains(t, err.Error(), "500")
}

func TestHTTPClientStream_RecvMsg_Non200EmptyBody(t *testing.T) {
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestHTTPClientStream_HeaderBeforeSendIsFailedPrecondition(t *testing.T) {
	cs := &httpClientStream{ctx: context.Background()}
	_, err := cs.Header()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request not sent")
}

func TestHTTPClientStream_HeaderHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cs := &httpClientStream{
		ctx:             ctx,
		reqSent:         true,
		responseStarted: true,
		headersReady:    make(chan struct{}),
	}
	cancel()

	_, err := cs.Header()
	require.ErrorIs(t, err, context.Canceled)
}

func TestHTTPClientStream_RecvMsg_InvalidJSON(t *testing.T) {
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	})

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.Error(t, err)
}

func TestHTTPClientStream_RecvMsg_OutgoingMetadata(t *testing.T) {
	var receivedHeader string
	cs, _ := newTestClientStream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(MetadataHeaderPrefix + "trace-id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":"ok"}`))
	})

	ctx := metadata.WithOutContext(cs.ctx, metadata.Pairs("trace-id", "abc-123"))
	cs.ctx = ctx

	err := cs.SendMsg(&testMessage{Value: "req"})
	require.NoError(t, err)

	var msg testMessage
	err = cs.RecvMsg(&msg)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", receivedHeader)
}

func TestHTTPClientStream_RecvMsg_AlreadyReceived(t *testing.T) {
	cs := &httpClientStream{
		ctx:      context.Background(),
		respRecv: true,
	}
	var msg testMessage
	err := cs.RecvMsg(&msg)
	require.NoError(t, nil)
	_ = err
}

func newRoundTripClientStream(
	transport http.RoundTripper,
	statsHandler stats.Handler,
) *httpClientStream {
	return &httpClientStream{
		ctx:                context.Background(),
		method:             "/test.Service/Method",
		endpointAddr:       "http://example.test",
		httpClient:         &http.Client{Transport: transport},
		statsHandler:       statsHandler,
		configuredOutbound: marshaler.NewJSONPbMarshalerWithConfig(nil),
		configuredInbound:  marshaler.NewJSONPbMarshalerWithConfig(nil),
	}
}

func successfulRoundTrip(responseBody string) roundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{MetadataHeaderPrefix + "X-Custom": {"custom-value"}},
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}, nil
	}
}

func TestHTTPClientStream_HeaderExecutesResponseOnce(t *testing.T) {
	var requests atomic.Int32
	statsHandler := &countingStatsHandler{}
	transport := successfulRoundTrip(`{"value":"resp"}`)
	cs := newRoundTripClientStream(
		roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests.Add(1)
			return transport(request)
		}),
		statsHandler,
	)

	require.NoError(t, cs.SendMsg(&testMessage{Value: "req"}))
	header, err := cs.Header()
	require.NoError(t, err)
	assert.Equal(t, []string{"custom-value"}, header.Get("X-Custom"))

	var response testMessage
	require.NoError(t, cs.RecvMsg(&response))
	assert.Equal(t, "resp", response.Value)
	assert.Equal(t, int32(1), requests.Load())
	assert.Equal(t, int32(1), statsHandler.begins.Load())
	assert.Equal(t, int32(1), statsHandler.outPayloads.Load())
	assert.Equal(t, int32(1), statsHandler.inPayloads.Load())
	assert.Equal(t, int32(1), statsHandler.ends.Load())
}

func TestHTTPClientStream_HeaderAndRecvShareResponse(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int32
	transport := successfulRoundTrip(`{"value":"shared"}`)
	cs := newRoundTripClientStream(
		roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if requests.Add(1) == 1 {
				close(started)
			}
			<-release
			return transport(request)
		}),
		stats.NoOpHandler,
	)

	require.NoError(t, cs.SendMsg(&testMessage{Value: "req"}))

	headerResult := make(chan error, 1)
	go func() {
		_, err := cs.Header()
		headerResult <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("response request did not start")
	}

	var response testMessage
	recvResult := make(chan error, 1)
	go func() {
		recvResult <- cs.RecvMsg(&response)
	}()
	close(release)

	select {
	case err := <-headerResult:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Header did not complete")
	}
	select {
	case err := <-recvResult:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("RecvMsg did not complete")
	}
	assert.Equal(t, "shared", response.Value)
	assert.Equal(t, int32(1), requests.Load())
}

func BenchmarkHTTPClientStreamUnary(b *testing.B) {
	for _, size := range []int{1 << 10, 64 << 10, 1 << 20} {
		responseBody := []byte(fmt.Sprintf(`{"value":"%s"}`, strings.Repeat("x", size)))
		b.Run(fmt.Sprintf("bytes=%d/recv", size), func(b *testing.B) {
			benchmarkHTTPClientStreamUnary(b, responseBody, false)
		})
		b.Run(fmt.Sprintf("bytes=%d/header-then-recv", size), func(b *testing.B) {
			benchmarkHTTPClientStreamUnary(b, responseBody, true)
		})
	}
}

func benchmarkHTTPClientStreamUnary(b *testing.B, responseBody []byte, headerFirst bool) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(responseBody)),
		}, nil
	})
	request := &testMessage{Value: "req"}
	target := &testMessage{}

	b.ReportAllocs()
	b.SetBytes(int64(len(responseBody)))
	b.ResetTimer()
	for range b.N {
		cs := newRoundTripClientStream(transport, stats.NoOpHandler)
		if err := cs.SendMsg(request); err != nil {
			b.Fatal(err)
		}
		if headerFirst {
			if _, err := cs.Header(); err != nil {
				b.Fatal(err)
			}
		}
		if err := cs.RecvMsg(target); err != nil {
			b.Fatal(err)
		}
	}
}

func TestAttachPeer_InvalidRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "justahost"
	ctx := transportmeta.AttachPeer(context.Background(), req, nil, nil)
	p, ok := peer.FromContext(ctx)
	require.True(t, ok)
	require.NotNil(t, p)
	// When RemoteAddr has no port, SplitHostPort fails, host="justahost", port=0.
	// net.ParseIP("justahost") returns nil, so Addr is &net.TCPAddr{IP: nil, Port: 0}.
	// The key assertion is that there is no panic.
	assert.Equal(t, 0, p.Addr.(*net.TCPAddr).Port)
}
