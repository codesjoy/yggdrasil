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
	"strconv"
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
	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const (
	// MetadataHeaderPrefix is the prefix for metadata headers.
	MetadataHeaderPrefix = "Yggdrasil-Metadata-"
	// MetadataTrailerPrefix is the prefix for metadata trailers.
	MetadataTrailerPrefix = "Yggdrasil-Trailer-"
)

type httpClientStream struct {
	ctx context.Context

	method        string
	endpointAddr  string
	defaultScheme string
	httpClient    *http.Client

	mu     sync.Mutex
	recvMu sync.Mutex

	reqMarshaler  marshaler.Marshaler
	respMarshaler marshaler.Marshaler

	beginTime  time.Time
	reqPayload any
	reqBytes   []byte
	reqSent    bool

	respRecv        bool
	responseStarted bool
	responseReady   bool
	responseStatus  int
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
	cs.respMarshaler = cs.configuredInbound
	if cs.respMarshaler == nil {
		cs.respMarshaler = marshaler.MarshalerForValue(m)
	}
	respMarshaler := cs.respMarshaler
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
	body := append([]byte(nil), cs.responseBody...)
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
	statsHandler.HandleRPC(ctx, &stats.RPCInPayloadBase{
		Client:        true,
		Payload:       nil,
		Data:          body,
		TransportSize: len(body),
		RecvTime:      time.Now(),
		Protocol:      Protocol,
	})

	if umErr := respMarshaler.Unmarshal(body, m); umErr != nil {
		cs.mu.Lock()
		cs.respErr = umErr
		cs.respRecv = true
		cs.mu.Unlock()
		statsHandler.HandleRPC(
			ctx,
			&stats.RPCEndBase{
				Client:    true,
				BeginTime: beginTime,
				EndTime:   time.Now(),
				Err:       umErr,
				Protocol:  Protocol,
			},
		)
		return umErr
	}

	cs.mu.Lock()
	cs.respErr = nil
	cs.respRecv = true
	cs.mu.Unlock()
	statsHandler.HandleRPC(
		ctx,
		&stats.RPCEndBase{
			Client:    true,
			BeginTime: beginTime,
			EndTime:   time.Now(),
			Err:       nil,
			Protocol:  Protocol,
		},
	)
	return nil
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
		go cs.executeResponse()
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

func (cs *httpClientStream) executeResponse() {
	cs.mu.Lock()
	reqBytes := append([]byte(nil), cs.reqBytes...)
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
	cs.mu.Unlock()

	if statsHandler == nil {
		statsHandler = stats.NoOpHandler
	}
	if beginTime.IsZero() {
		beginTime = time.Now()
	}
	statsHandler.HandleRPC(ctx, &stats.RPCBeginBase{
		Client:       true,
		BeginTime:    beginTime,
		ClientStream: false,
		ServerStream: false,
		Protocol:     Protocol,
	})

	url := endpointAddr
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		scheme := defaultScheme
		if scheme == "" {
			scheme = "http"
		}
		url = scheme + "://" + strings.TrimRight(url, "/")
	} else {
		url = strings.TrimSuffix(url, "/")
	}
	url += method

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		cs.finishResponse(nil, nil, http.StatusInternalServerError, nil, err, ch)
		return
	}
	req.Header.Set("Content-Type", reqMarshaler.ContentType(nil))
	req.Header.Set("Accept", respMarshaler.ContentType(nil))
	req.Header.Set("TE", "trailers")
	for key, values := range outMD {
		for _, value := range values {
			req.Header.Add(MetadataHeaderPrefix+key, value)
		}
	}

	statsHandler.HandleRPC(ctx, &stats.OutHeaderBase{
		Client:         true,
		Header:         outMD,
		FullMethod:     method,
		RemoteEndpoint: endpointAddr,
		Protocol:       Protocol,
	})
	statsHandler.HandleRPC(ctx, &stats.RPCOutPayloadBase{
		Client:        true,
		Payload:       reqPayload,
		Data:          reqBytes,
		TransportSize: len(reqBytes),
		SendTime:      time.Now(),
		Protocol:      Protocol,
	})

	resp, err := hc.Do(req)
	if err != nil {
		cs.finishResponse(nil, nil, 0, nil, err, ch)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	headerMD := extractMetadataWithPrefix(resp.Header, MetadataHeaderPrefix)
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		cs.finishResponse(headerMD, nil, resp.StatusCode, nil, readErr, ch)
		return
	}
	trailerMD := extractMetadataWithPrefix(resp.Trailer, MetadataTrailerPrefix)
	if resp.StatusCode != http.StatusOK {
		var pb stpb.Status
		unmarshalErr := respMarshaler.Unmarshal(body, &pb)
		if unmarshalErr != nil || pb.Code == int32(code.Code_OK) {
			err = xerror.New(
				status.HTTPCodeToStuCode(int32(resp.StatusCode)),
				fmt.Sprintf("http status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			)
		} else {
			err = status.FromProto(&pb).Err()
		}
		cs.finishResponse(headerMD, trailerMD, resp.StatusCode, nil, err, ch)
		statsHandler.HandleRPC(ctx, &stats.RPCEndBase{
			Client:    true,
			BeginTime: beginTime,
			EndTime:   time.Now(),
			Err:       err,
			Protocol:  Protocol,
		})
		return
	}
	cs.finishResponse(headerMD, trailerMD, resp.StatusCode, body, nil, ch)
}

func (cs *httpClientStream) finishResponse(
	h metadata.MD,
	t metadata.MD,
	statusCode int,
	body []byte,
	err error,
	ch chan struct{},
) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.header = h
	cs.trailer = t
	cs.responseStatus = statusCode
	cs.responseBody = append([]byte(nil), body...)
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

func (cs *httpClientStream) finish(h, t metadata.MD, err error, ch chan struct{}) {
	cs.finishResponse(h, t, http.StatusOK, nil, err, ch)
	cs.mu.Lock()
	cs.respRecv = true
	cs.mu.Unlock()
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
			Header:   extractMetadataWithPrefix(ss.req.Header, MetadataHeaderPrefix),
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

	writeMetadata(w, inMD)

	statsHandler.HandleRPC(ss.ctx, &stats.OutHeaderBase{
		Client:        false,
		Header:        inMD,
		Protocol:      Protocol,
		TransportSize: 0,
	})

	doTrailers := requestAcceptsTrailers(req)
	if doTrailers && trMD.Len() > 0 {
		declareTrailers(w, trMD)
		w.Header().Del("Content-Length")
	}

	if err != nil {
		st := status.FromError(err)
		pb := st.Status()
		buf, mErr := outbound.Marshal(pb)
		if mErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":13,"message":"failed to marshal error message"}`))
			return
		}
		w.Header().Set("Content-Type", outbound.ContentType(pb))
		if st.IsCode(code.Code_UNAUTHENTICATED) {
			w.Header().Set("WWW-Authenticate", st.Message())
		}
		w.WriteHeader(int(st.HTTPCode()))
		_, _ = w.Write(buf)
		statsHandler.HandleRPC(ss.ctx, &stats.RPCOutPayloadBase{
			Client:        false,
			Payload:       pb,
			Data:          buf,
			TransportSize: len(buf),
			SendTime:      time.Now(),
			Protocol:      Protocol,
		})
		if doTrailers && trMD.Len() > 0 {
			writeTrailers(w, trMD)
			statsHandler.HandleRPC(ss.ctx, &stats.OutTrailerBase{Client: false, Trailer: trMD})
		}
		statsHandler.HandleRPC(
			ss.ctx,
			&stats.RPCEndBase{
				Client:    false,
				BeginTime: beginTime,
				EndTime:   time.Now(),
				Err:       err,
				Protocol:  Protocol,
			},
		)
		return
	}

	if reply == nil {
		w.WriteHeader(http.StatusOK)
		if doTrailers && trMD.Len() > 0 {
			writeTrailers(w, trMD)
			statsHandler.HandleRPC(ss.ctx, &stats.OutTrailerBase{Client: false, Trailer: trMD})
		}
		statsHandler.HandleRPC(
			ss.ctx,
			&stats.RPCEndBase{
				Client:    false,
				BeginTime: beginTime,
				EndTime:   time.Now(),
				Err:       nil,
				Protocol:  Protocol,
			},
		)
		return
	}

	if pm, ok := reply.(proto.Message); ok {
		w.Header().Set("Content-Type", outbound.ContentType(pm))
	}
	buf, mErr := outbound.Marshal(reply)
	if mErr != nil {
		st := status.FromError(mErr)
		pb := st.Status()
		w.Header().Set("Content-Type", outbound.ContentType(pb))
		w.WriteHeader(int(st.HTTPCode()))
		if b, e2 := outbound.Marshal(pb); e2 == nil {
			_, _ = w.Write(b)
			statsHandler.HandleRPC(ss.ctx, &stats.RPCOutPayloadBase{
				Client:        false,
				Payload:       pb,
				Data:          b,
				TransportSize: len(b),
				SendTime:      time.Now(),
				Protocol:      Protocol,
			})
		}
		if doTrailers && trMD.Len() > 0 {
			writeTrailers(w, trMD)
			statsHandler.HandleRPC(ss.ctx, &stats.OutTrailerBase{Client: false, Trailer: trMD})
		}
		statsHandler.HandleRPC(
			ss.ctx,
			&stats.RPCEndBase{
				Client:    false,
				BeginTime: beginTime,
				EndTime:   time.Now(),
				Err:       mErr,
				Protocol:  Protocol,
			},
		)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
	statsHandler.HandleRPC(ss.ctx, &stats.RPCOutPayloadBase{
		Client:        false,
		Payload:       reply,
		Data:          buf,
		TransportSize: len(buf),
		SendTime:      time.Now(),
		Protocol:      Protocol,
	})
	if doTrailers && trMD.Len() > 0 {
		writeTrailers(w, trMD)
		statsHandler.HandleRPC(ss.ctx, &stats.OutTrailerBase{Client: false, Trailer: trMD})
	}
	statsHandler.HandleRPC(
		ss.ctx,
		&stats.RPCEndBase{
			Client:    false,
			BeginTime: beginTime,
			EndTime:   time.Now(),
			Err:       nil,
			Protocol:  Protocol,
		},
	)
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

func extractMetadataWithPrefix(h http.Header, prefix string) metadata.MD {
	md := metadata.MD{}
	for key, vals := range h {
		if strings.HasPrefix(key, prefix) {
			md.Append(key[len(prefix):], vals...)
		}
	}
	return md
}

func writeMetadata(w http.ResponseWriter, md metadata.MD) {
	for k, vs := range md {
		for _, v := range vs {
			w.Header().Add(MetadataHeaderPrefix+k, v)
		}
	}
}

func requestAcceptsTrailers(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.Header.Get("TE")), "trailers")
}

func declareTrailers(w http.ResponseWriter, md metadata.MD) {
	for k := range md {
		w.Header().Add("Trailer", MetadataTrailerPrefix+k)
	}
}

func writeTrailers(w http.ResponseWriter, md metadata.MD) {
	for k, vs := range md {
		for _, v := range vs {
			w.Header().Add(MetadataTrailerPrefix+k, v)
		}
	}
}

func attachPeer(
	ctx context.Context,
	r *http.Request,
	localAddr net.Addr,
	authInfo security.AuthInfo,
) context.Context {
	host, portStr, err := net.SplitHostPort(r.RemoteAddr)
	port := 0
	if err == nil {
		port, _ = strconv.Atoi(portStr)
	} else {
		host = r.RemoteAddr
	}
	p := &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   net.ParseIP(host),
			Port: port,
		},
		LocalAddr: localAddr,
		AuthInfo:  authInfo,
		Protocol:  "http",
	}
	return peer.WithContext(ctx, p)
}

var _ stream.ServerStream = (*httpServerStream)(nil)
