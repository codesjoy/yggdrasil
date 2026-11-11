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
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"google.golang.org/genproto/googleapis/rpc/code"
	stpb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	"github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	"github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	transportmeta "github.com/codesjoy/yggdrasil/v3/transport/internal/httpmeta"
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

const (
	// MetadataHeaderPrefix is the prefix for metadata headers.
	MetadataHeaderPrefix = transportmeta.MetadataHeaderPrefix
	// MetadataTrailerPrefix is the prefix for metadata trailers.
	MetadataTrailerPrefix = transportmeta.MetadataTrailerPrefix
)

type httpClientStream struct {
	ctx context.Context

	method        string
	endpointAddr  string
	defaultScheme string
	httpClient    *http.Client

	mu     sync.Mutex
	recvMu sync.Mutex

	reqMarshaler marshaler.Marshaler

	beginTime  time.Time
	reqPayload any
	reqBytes   []byte
	reqSent    bool

	respRecv        bool
	responseStarted bool
	responseReady   bool
	responseBody    []byte
	header          metadata.MD
	trailer         metadata.MD
	respErr         error
	headersReady    chan struct{}
	statsHandler    stats.Handler

	configuredInbound  marshaler.Marshaler
	configuredOutbound marshaler.Marshaler
}

func (cs *httpClientStream) Header() (metadata.MD, error) {
	cs.mu.Lock()
	if !cs.reqSent {
		cs.mu.Unlock()
		return nil, xerror.New(code.Code_FAILED_PRECONDITION, "request not sent")
	}
	already := cs.respRecv || cs.responseReady
	err := cs.respErr
	h := cs.header
	cs.mu.Unlock()

	if already {
		return h.Copy(), err
	}
	if err := cs.ensureResponse(); err != nil {
		cs.mu.Lock()
		header := cs.header
		cs.mu.Unlock()
		if header == nil {
			header = metadata.MD{}
		}
		return header.Copy(), err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.header == nil {
		return metadata.MD{}, cs.respErr
	}
	return cs.header.Copy(), cs.respErr
}

func (cs *httpClientStream) Trailer() metadata.MD {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.trailer == nil {
		return metadata.MD{}
	}
	return cs.trailer.Copy()
}

func (cs *httpClientStream) CloseSend() error {
	return nil
}

func (cs *httpClientStream) Context() context.Context {
	return cs.ctx
}

func (cs *httpClientStream) SendMsg(m interface{}) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.reqSent {
		return xerror.New(code.Code_FAILED_PRECONDITION, "message already sent")
	}
	cs.reqMarshaler = cs.configuredOutbound
	if cs.reqMarshaler == nil {
		cs.reqMarshaler = marshaler.MarshalerForValue(m)
	}
	cs.reqPayload = m
	cs.reqBytes, cs.respErr = cs.reqMarshaler.Marshal(m)
	if cs.respErr != nil {
		return cs.respErr
	}
	cs.reqSent = true
	return nil
}

func (cs *httpClientStream) RecvMsg(m interface{}) error {
	cs.recvMu.Lock()
	defer cs.recvMu.Unlock()

	cs.mu.Lock()
	if cs.respRecv {
		err := cs.respErr
		cs.mu.Unlock()
		return err
	}
	if !cs.reqSent {
		cs.mu.Unlock()
		return xerror.New(code.Code_FAILED_PRECONDITION, "request not sent")
	}
	respMarshaler := cs.configuredInbound
	if respMarshaler == nil {
		respMarshaler = marshaler.MarshalerForValue(m)
	}
	cs.mu.Unlock()

	if err := cs.ensureResponse(); err != nil {
		cs.mu.Lock()
		cs.respRecv = true
		cs.mu.Unlock()
		return err
	}

	cs.mu.Lock()
	if cs.respRecv {
		err := cs.respErr
		cs.mu.Unlock()
		return err
	}
	body := cs.responseBody
	ctx := cs.ctx
	beginTime := cs.beginTime
	statsHandler := cs.statsHandler
	cs.mu.Unlock()

	if statsHandler == nil {
		statsHandler = stats.NoOpHandler
	}
	if beginTime.IsZero() {
		beginTime = time.Now()
	}
	return cs.decodeResponse(m, respMarshaler, body, ctx, beginTime, statsHandler)
}

func (cs *httpClientStream) decodeResponse(
	target any,
	responseMarshaler marshaler.Marshaler,
	body []byte,
	ctx context.Context,
	beginTime time.Time,
	statsHandler stats.Handler,
) error {
	statsHandler.HandleRPC(ctx, &stats.RPCInPayloadBase{
		Client:        true,
		Payload:       nil,
		Data:          body,
		TransportSize: len(body),
		RecvTime:      time.Now(),
		Protocol:      Protocol,
	})

	if err := responseMarshaler.Unmarshal(body, target); err != nil {
		cs.mu.Lock()
		cs.respErr = err
		cs.respRecv = true
		cs.mu.Unlock()
		handleClientReceiveEnd(ctx, statsHandler, beginTime, err)
		return err
	}

	cs.mu.Lock()
	cs.respErr = nil
	cs.respRecv = true
	cs.mu.Unlock()
	handleClientReceiveEnd(ctx, statsHandler, beginTime, nil)
	return nil
}

func handleClientReceiveEnd(
	ctx context.Context,
	statsHandler stats.Handler,
	beginTime time.Time,
	err error,
) {
	statsHandler.HandleRPC(
		ctx,
		&stats.RPCEndBase{
			Client:    true,
			BeginTime: beginTime,
			EndTime:   time.Now(),
			Err:       err,
			Protocol:  Protocol,
		},
	)
}

func (cs *httpClientStream) ensureResponse() error {
	cs.mu.Lock()
	if !cs.reqSent {
		cs.mu.Unlock()
		return xerror.New(code.Code_FAILED_PRECONDITION, "request not sent")
	}
	if cs.responseReady {
		err := cs.respErr
		cs.mu.Unlock()
		return err
	}
	if cs.headersReady == nil {
		cs.headersReady = make(chan struct{})
	}
	if !cs.responseStarted {
		cs.responseStarted = true
		execution := cs.responseExecutionLocked()
		cs.mu.Unlock()
		cs.executeResponse(execution)
		cs.mu.Lock()
		err := cs.respErr
		cs.mu.Unlock()
		return err
	}
	ready := cs.headersReady
	ctx := cs.ctx
	cs.mu.Unlock()

	select {
	case <-ready:
		cs.mu.Lock()
		err := cs.respErr
		cs.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cs *httpClientStream) executeResponse(execution responseExecution) {
	execution.statsHandler.HandleRPC(execution.ctx, &stats.RPCBeginBase{
		Client:       true,
		BeginTime:    execution.beginTime,
		ClientStream: false,
		ServerStream: false,
		Protocol:     Protocol,
	})

	request, err := execution.newRequest()
	if err != nil {
		cs.finishResponse(nil, nil, nil, err, execution.headersReady)
		return
	}
	response, err := execution.httpClient.Do(request)
	if err != nil {
		cs.finishResponse(nil, nil, nil, err, execution.headersReady)
		return
	}
	defer response.Body.Close() //nolint:errcheck
	cs.readResponse(execution, response)
}

type responseExecution struct {
	ctx           context.Context
	method        string
	endpointAddr  string
	defaultScheme string
	httpClient    *http.Client
	reqMarshaler  marshaler.Marshaler
	respMarshaler marshaler.Marshaler
	beginTime     time.Time
	reqPayload    any
	reqBytes      []byte
	outMD         metadata.MD
	statsHandler  stats.Handler
	headersReady  chan struct{}
}

func (cs *httpClientStream) responseExecutionLocked() responseExecution {
	reqBytes := cs.reqBytes
	reqMarshaler := cs.reqMarshaler
	respMarshaler := cs.configuredInbound
	if respMarshaler == nil {
		respMarshaler = marshaler.MarshalerForValue(&stpb.Status{})
	}
	endpointAddr := cs.endpointAddr
	method := cs.method
	hc := cs.httpClient
	ctx := cs.ctx
	ch := cs.headersReady
	beginTime := cs.beginTime
	statsHandler := cs.statsHandler
	outMD, _ := metadata.FromOutContext(ctx)
	reqPayload := cs.reqPayload
	defaultScheme := cs.defaultScheme

	if statsHandler == nil {
		statsHandler = stats.NoOpHandler
	}
	if beginTime.IsZero() {
		beginTime = time.Now()
	}
	return responseExecution{
		ctx:           ctx,
		method:        method,
		endpointAddr:  endpointAddr,
		defaultScheme: defaultScheme,
		httpClient:    hc,
		reqMarshaler:  reqMarshaler,
		respMarshaler: respMarshaler,
		beginTime:     beginTime,
		reqPayload:    reqPayload,
		reqBytes:      reqBytes,
		outMD:         outMD,
		statsHandler:  statsHandler,
		headersReady:  ch,
	}
}

func (execution responseExecution) newRequest() (*http.Request, error) {
	url := execution.endpointAddr
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		scheme := execution.defaultScheme
		if scheme == "" {
			scheme = "http"
		}
		url = scheme + "://" + strings.TrimRight(url, "/")
	} else {
		url = strings.TrimSuffix(url, "/")
	}
	url += execution.method

	req, err := http.NewRequestWithContext(
		execution.ctx,
		http.MethodPost,
		url,
		bytes.NewReader(execution.reqBytes),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", execution.reqMarshaler.ContentType(nil))
	req.Header.Set("Accept", execution.respMarshaler.ContentType(nil))
	req.Header.Set("TE", "trailers")
	for key, values := range execution.outMD {
		for _, value := range values {
			req.Header.Add(MetadataHeaderPrefix+key, value)
		}
	}

	execution.statsHandler.HandleRPC(execution.ctx, &stats.OutHeaderBase{
		Client:         true,
		Header:         execution.outMD,
		FullMethod:     execution.method,
		RemoteEndpoint: execution.endpointAddr,
		Protocol:       Protocol,
	})
	execution.statsHandler.HandleRPC(execution.ctx, &stats.RPCOutPayloadBase{
		Client:        true,
		Payload:       execution.reqPayload,
		Data:          execution.reqBytes,
		TransportSize: len(execution.reqBytes),
		SendTime:      time.Now(),
		Protocol:      Protocol,
	})
	return req, nil
}

func (cs *httpClientStream) readResponse(
	execution responseExecution,
	resp *http.Response,
) {
	headerMD := transportmeta.ExtractMetadata(resp.Header, MetadataHeaderPrefix)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cs.finishResponse(headerMD, nil, nil, err, execution.headersReady)
		return
	}
	trailerMD := transportmeta.ExtractMetadata(resp.Trailer, MetadataTrailerPrefix)
	if err = execution.statusError(resp.StatusCode, body); err != nil {
		cs.finishResponse(headerMD, trailerMD, nil, err, execution.headersReady)
		execution.statsHandler.HandleRPC(execution.ctx, &stats.RPCEndBase{
			Client:    true,
			BeginTime: execution.beginTime,
			EndTime:   time.Now(),
			Err:       err,
			Protocol:  Protocol,
		})
		return
	}
	cs.finishResponse(headerMD, trailerMD, body, nil, execution.headersReady)
}

func (execution responseExecution) statusError(statusCode int, body []byte) error {
	if statusCode == http.StatusOK {
		return nil
	}
	var pb stpb.Status
	unmarshalErr := execution.respMarshaler.Unmarshal(body, &pb)
	if unmarshalErr != nil || pb.Code == int32(code.Code_OK) {
		return xerror.New(
			status.HTTPCodeToStuCode(int32(statusCode)),
			fmt.Sprintf("http status %d %s", statusCode, http.StatusText(statusCode)),
		)
	}
	return status.FromProto(&pb).Err()
}

func (cs *httpClientStream) finishResponse(
	h metadata.MD,
	t metadata.MD,
	body []byte,
	err error,
	ch chan struct{},
) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.header = h
	cs.trailer = t
	cs.responseBody = body
	cs.respErr = err
	cs.responseReady = true
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

type httpServerStream struct {
	ctx context.Context

	method             string
	req                *http.Request
	w                  http.ResponseWriter
	localAddr          net.Addr
	maxBodyBytes       int64
	statsHandler       stats.Handler
	beginTime          time.Time
	remoteEndpoint     string
	localEndpoint      string
	configuredInbound  marshaler.Marshaler
	configuredOutbound marshaler.Marshaler

	mu sync.Mutex

	started bool

	inbound  marshaler.Marshaler
	outbound marshaler.Marshaler

	reqBody []byte
	recv    bool

	headerMD  metadata.MD
	trailerMD metadata.MD

	finished bool
}

func (ss *httpServerStream) Method() string {
	return ss.method
}

func (ss *httpServerStream) Start(isClientStream, isServerStream bool) error {
	if isClientStream || isServerStream {
		return xerror.New(code.Code_UNIMPLEMENTED, "http protocol does not support streaming")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.started {
		return xerror.New(code.Code_FAILED_PRECONDITION, "stream already started")
	}
	ss.started = true
	ss.inbound = ss.configuredInbound
	if ss.inbound == nil {
		ss.inbound = selectInboundMarshaler(nil, ss.req.Header.Get("Content-Type"))
	}
	accept := ss.req.Header.Get("Accept")
	ss.outbound = selectOutboundMarshaler(ss.configuredOutbound, accept, ss.inbound)

	limit := ss.maxBodyBytes
	if limit <= 0 {
		limit = 4 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(ss.req.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > limit {
		return xerror.New(code.Code_RESOURCE_EXHAUSTED, "request body too large")
	}
	ss.reqBody = body

	ss.statsHandler.HandleRPC(ss.ctx, &stats.RPCBeginBase{
		Client:       false,
		BeginTime:    ss.beginTime,
		ClientStream: false,
		ServerStream: false,
		Protocol:     Protocol,
	})
	ss.statsHandler.HandleRPC(ss.ctx, &stats.RPCServerInHeaderBase{
		RPCInHeaderBase: stats.RPCInHeaderBase{
			Header: transportmeta.ExtractMetadata(
				ss.req.Header,
				MetadataHeaderPrefix,
			),
			Protocol: Protocol,
		},
		FullMethod:     ss.method,
		RemoteEndpoint: ss.remoteEndpoint,
		LocalEndpoint:  ss.localEndpoint,
	})
	ss.statsHandler.HandleRPC(ss.ctx, &stats.RPCInPayloadBase{
		Client:        false,
		Payload:       nil,
		Data:          body,
		TransportSize: len(body),
		RecvTime:      time.Now(),
		Protocol:      Protocol,
	})
	return nil
}

func (ss *httpServerStream) Finish(reply any, err error) {
	ss.mu.Lock()
	if ss.finished {
		ss.mu.Unlock()
		return
	}
	ss.finished = true

	inMD := ss.headerMD
	trMD := ss.trailerMD
	outbound := ss.outbound
	req := ss.req
	w := ss.w
	beginTime := ss.beginTime
	statsHandler := ss.statsHandler
	ss.mu.Unlock()

	transportmeta.WriteMetadata(w, inMD, transportmeta.PrefixMatcher(MetadataHeaderPrefix))

	statsHandler.HandleRPC(ss.ctx, &stats.OutHeaderBase{
		Client:        false,
		Header:        inMD,
		Protocol:      Protocol,
		TransportSize: 0,
	})

	forwardTrailers := prepareServerTrailers(w, req, trMD)
	endErr := err
	if err != nil {
		if !writeServerError(ss.ctx, w, statsHandler, outbound, err) {
			return
		}
	} else {
		endErr = writeServerReply(ss.ctx, w, statsHandler, outbound, reply)
	}

	if forwardTrailers {
		transportmeta.WriteTrailers(w, trMD, transportmeta.PrefixMatcher(MetadataTrailerPrefix))
		statsHandler.HandleRPC(ss.ctx, &stats.OutTrailerBase{Client: false, Trailer: trMD})
	}
	statsHandler.HandleRPC(ss.ctx, &stats.RPCEndBase{
		Client:    false,
		BeginTime: beginTime,
		EndTime:   time.Now(),
		Err:       endErr,
		Protocol:  Protocol,
	})
}

func prepareServerTrailers(w http.ResponseWriter, req *http.Request, trailer metadata.MD) bool {
	if !transportmeta.AcceptsTrailers(req) || trailer.Len() == 0 {
		return false
	}
	transportmeta.DeclareTrailers(
		w,
		trailer,
		transportmeta.PrefixMatcher(MetadataTrailerPrefix),
	)
	w.Header().Del("Content-Length")
	return true
}

func writeServerError(
	ctx context.Context,
	w http.ResponseWriter,
	statsHandler stats.Handler,
	outbound marshaler.Marshaler,
	err error,
) bool {
	st := status.FromError(err)
	pb := st.Status()
	body, marshalErr := outbound.Marshal(pb)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":13,"message":"failed to marshal error message"}`))
		return false
	}
	w.Header().Set("Content-Type", outbound.ContentType(pb))
	if st.IsCode(code.Code_UNAUTHENTICATED) {
		w.Header().Set("WWW-Authenticate", st.Message())
	}
	w.WriteHeader(int(st.HTTPCode()))
	_, _ = w.Write(body)
	emitServerPayload(ctx, statsHandler, pb, body)
	return true
}

func writeServerReply(
	ctx context.Context,
	w http.ResponseWriter,
	statsHandler stats.Handler,
	outbound marshaler.Marshaler,
	reply any,
) error {
	if reply == nil {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	if message, ok := reply.(proto.Message); ok {
		w.Header().Set("Content-Type", outbound.ContentType(message))
	}
	body, err := outbound.Marshal(reply)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		emitServerPayload(ctx, statsHandler, reply, body)
		return nil
	}

	responseStatus := status.FromError(err)
	errorStatus := responseStatus.Status()
	w.Header().Set("Content-Type", outbound.ContentType(errorStatus))
	w.WriteHeader(int(responseStatus.HTTPCode()))
	if body, marshalErr := outbound.Marshal(errorStatus); marshalErr == nil {
		_, _ = w.Write(body)
		emitServerPayload(ctx, statsHandler, errorStatus, body)
	}
	return err
}

func emitServerPayload(
	ctx context.Context,
	statsHandler stats.Handler,
	payload any,
	body []byte,
) {
	statsHandler.HandleRPC(ctx, &stats.RPCOutPayloadBase{
		Client:        false,
		Payload:       payload,
		Data:          body,
		TransportSize: len(body),
		SendTime:      time.Now(),
		Protocol:      Protocol,
	})
}

func (ss *httpServerStream) SetHeader(md metadata.MD) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.headerMD = metadata.Join(ss.headerMD, md)
	return nil
}

func (ss *httpServerStream) SendHeader(md metadata.MD) error {
	return ss.SetHeader(md)
}

func (ss *httpServerStream) SetTrailer(md metadata.MD) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.trailerMD = metadata.Join(ss.trailerMD, md)
}

func (ss *httpServerStream) Context() context.Context {
	return ss.ctx
}

func (ss *httpServerStream) SendMsg(m any) error {
	ss.Finish(m, nil)
	return nil
}

func (ss *httpServerStream) RecvMsg(m any) error {
	ss.mu.Lock()
	if ss.recv {
		ss.mu.Unlock()
		return io.EOF
	}
	ss.recv = true
	body := ss.reqBody
	inbound := ss.inbound
	ss.mu.Unlock()
	if len(body) == 0 {
		return nil
	}
	return inbound.Unmarshal(body, m)
}

var _ stream.ServerStream = (*httpServerStream)(nil)
