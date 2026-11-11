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
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	ginsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/interop/grpc_testing"
	gmetadata "google.golang.org/grpc/metadata"
	gpeer "google.golang.org/grpc/peer"
	gstatus "google.golang.org/grpc/status"

	"github.com/codesjoy/yggdrasil/v3/discovery/resolver"
	"github.com/codesjoy/yggdrasil/v3/observability/stats"
	ymetadata "github.com/codesjoy/yggdrasil/v3/rpc/metadata"
	ystatus "github.com/codesjoy/yggdrasil/v3/rpc/status"
	"github.com/codesjoy/yggdrasil/v3/rpc/stream"
	remote "github.com/codesjoy/yggdrasil/v3/transport"
	"github.com/codesjoy/yggdrasil/v3/transport/support/peer"
	"github.com/codesjoy/yggdrasil/v3/transport/support/security"
)

const (
	interopSlowMethod = "/grpc.testing.TestService/SlowCall"
)

type interopReason struct{}

func (interopReason) Reason() string  { return "INTEROP_FAILURE" }
func (interopReason) Domain() string  { return "transport.grpc.interop" }
func (interopReason) Code() code.Code { return code.Code_INVALID_ARGUMENT }

type runningYGGServer struct {
	remote.Server
	errCh    chan error
	stopOnce sync.Once
	waitOnce sync.Once
	stopErr  error
}

func startYGGServer(
	t *testing.T,
	settings Settings,
	handle remote.MethodHandle,
	profiles map[string]security.Profile,
) *runningYGGServer {
	t.Helper()
	provider := ServerProviderWithSettings(settings, stats.NoOpHandler, profiles)
	srv, err := provider.NewServer(handle)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Handle()
	}()
	running := &runningYGGServer{Server: srv, errCh: errCh}
	t.Cleanup(func() {
		running.stopAndWait(t)
	})
	return running
}

func (s *runningYGGServer) stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.stopOnce.Do(func() {
		s.stopErr = s.Stop(ctx)
	})
	return s.stopErr
}

func (s *runningYGGServer) wait(t *testing.T) {
	t.Helper()
	s.waitOnce.Do(func() {
		select {
		case err := <-s.errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("yggdrasil grpc server did not stop")
		}
	})
}

func (s *runningYGGServer) stopAndWait(t *testing.T) {
	t.Helper()
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.stop(stopCtx)
	s.wait(t)
}

func dialGRPCGo(t *testing.T, address string, opts ...ggrpc.DialOption) *ggrpc.ClientConn {
	t.Helper()
	if len(opts) == 0 {
		opts = append(opts, ggrpc.WithTransportCredentials(ginsecure.NewCredentials()))
	}
	conn, err := ggrpc.NewClient(address, opts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

func newYGGClient(
	t *testing.T,
	address string,
	compressor string,
	profiles map[string]security.Profile,
) remote.Client {
	t.Helper()
	return newYGGClientWithConfig(
		t,
		address,
		ClientConfig{Compressor: compressor},
		profiles,
	)
}

func newYGGClientWithConfig(
	t *testing.T,
	address string,
	clientConfig ClientConfig,
	profiles map[string]security.Profile,
) remote.Client {
	t.Helper()
	provider := ClientProviderWithSettings(Settings{
		Client: clientConfig,
	}, profiles)
	client, err := provider.NewClient(
		context.Background(),
		"grpc-interop",
		resolver.BaseEndpoint{Protocol: Protocol, Address: address},
		stats.NoOpHandler,
		nil,
	)
	require.NoError(t, err)
	client.Connect()
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func startGRPCGoServer(
	t *testing.T,
	impl grpc_testing.TestServiceServer,
	opts ...ggrpc.ServerOption,
) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := ggrpc.NewServer(opts...)
	grpc_testing.RegisterTestServiceServer(server, impl)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(lis)
	}()
	t.Cleanup(func() {
		server.Stop()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, ggrpc.ErrServerStopped) {
				t.Errorf("grpc-go server stopped with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("grpc-go server did not stop")
		}
	})
	return lis.Addr().String()
}

func handleYGGInterop(ss remote.ServerStream) {
	var reply any
	var err error
	defer func() {
		ss.Finish(reply, err)
	}()

	switch ss.Method() {
	case grpc_testing.TestService_UnaryCall_FullMethodName:
		err = ss.Start(false, false)
		if err != nil {
			return
		}
		req := new(grpc_testing.SimpleRequest)
		if err = ss.RecvMsg(req); err != nil {
			return
		}
		md, _ := ymetadata.FromInContext(ss.Context())
		p, _ := peer.FromContext(ss.Context())
		_ = ss.SetHeader(ymetadata.Pairs(
			"x-response", "one",
			"x-response", "two",
			"x-request-multi", strings.Join(md.Get("multi"), ","),
			"x-request-bin", firstMetadataValue(md, "trace-bin"),
			"x-user-agent", firstMetadataValue(md, "user-agent"),
		))
		ss.SetTrailer(ymetadata.Pairs("x-trailer", "done"))
		reply = &grpc_testing.SimpleResponse{
			Username: strings.Join(md.Get("multi"), ","),
			ServerId: p.RemoteIP,
		}
	case grpc_testing.TestService_StreamingOutputCall_FullMethodName:
		err = ss.Start(false, true)
		if err != nil {
			return
		}
		req := new(grpc_testing.StreamingOutputCallRequest)
		if err = ss.RecvMsg(req); err != nil {
			return
		}
		count := len(req.GetResponseParameters())
		if count == 0 {
			count = 2
		}
		for index := 0; index < count; index++ {
			err = ss.SendMsg(&grpc_testing.StreamingOutputCallResponse{
				Payload: &grpc_testing.Payload{Body: []byte("chunk")},
			})
			if err != nil {
				return
			}
		}
	case grpc_testing.TestService_StreamingInputCall_FullMethodName:
		err = ss.Start(true, false)
		if err != nil {
			return
		}
		var size int32
		for {
			req := new(grpc_testing.StreamingInputCallRequest)
			recvErr := ss.RecvMsg(req)
			if errors.Is(recvErr, io.EOF) {
				err = ss.SendMsg(&grpc_testing.StreamingInputCallResponse{
					AggregatedPayloadSize: size,
				})
				reply = nil
				return
			}
			if recvErr != nil {
				err = recvErr
				return
			}
			if payload := req.GetPayload(); payload != nil {
				size += int32(len(payload.GetBody()))
			}
		}
	case grpc_testing.TestService_FullDuplexCall_FullMethodName:
		err = ss.Start(true, true)
		if err != nil {
			return
		}
		for {
			req := new(grpc_testing.StreamingOutputCallRequest)
			recvErr := ss.RecvMsg(req)
			if errors.Is(recvErr, io.EOF) {
				return
			}
			if recvErr != nil {
				err = recvErr
				return
			}
			err = ss.SendMsg(&grpc_testing.StreamingOutputCallResponse{
				Payload: req.GetPayload(),
			})
			if err != nil {
				return
			}
		}
	case grpc_testing.TestService_UnimplementedCall_FullMethodName:
		err = ss.Start(false, false)
		if err != nil {
			return
		}
		ss.SetTrailer(ymetadata.Pairs("x-error-trailer", "present"))
		err = xerror.NewWithReason(
			interopReason{},
			"invalid interop request",
			map[string]string{"source": "yggdrasil"},
		)
	case interopSlowMethod:
		err = ss.Start(false, false)
		if err != nil {
			return
		}
		<-ss.Context().Done()
		err = ss.Context().Err()
	default:
		err = xerror.New(code.Code_UNIMPLEMENTED, "unknown interop method")
	}
}

type grpcGoInteropService struct {
	grpc_testing.UnimplementedTestServiceServer
}

func (grpcGoInteropService) UnaryCall(
	ctx context.Context,
	req *grpc_testing.SimpleRequest,
) (*grpc_testing.SimpleResponse, error) {
	if req.GetFillServerId() {
		<-ctx.Done()
		return nil, gstatus.FromContextError(ctx.Err()).Err()
	}
	if req.GetFillOauthScope() {
		st, err := gstatus.New(codes.InvalidArgument, "invalid grpc-go request").WithDetails(
			&errdetails.ErrorInfo{
				Reason:   "GRPC_GO_FAILURE",
				Domain:   "transport.grpc.interop",
				Metadata: map[string]string{"source": "grpc-go"},
			},
		)
		if err != nil {
			return nil, err
		}
		return nil, st.Err()
	}
	md, _ := gmetadata.FromIncomingContext(ctx)
	_ = ggrpc.SetHeader(ctx, gmetadata.Pairs(
		"x-response", "one",
		"x-response", "two",
		"x-request-multi", strings.Join(md.Get("multi"), ","),
		"x-request-bin", firstMetadataValue(fromGRPCMetadata(md), "trace-bin"),
		"x-user-agent", firstMetadataValue(fromGRPCMetadata(md), "user-agent"),
	))
	ggrpc.SetTrailer(ctx, gmetadata.Pairs("x-trailer", "done"))
	serverID := ""
	if p, ok := gpeer.FromContext(ctx); ok && p.Addr != nil {
		serverID = p.Addr.String()
	}
	return &grpc_testing.SimpleResponse{
		Username: strings.Join(md.Get("multi"), ","),
		ServerId: serverID,
	}, nil
}

func firstMetadataValue(md ymetadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (grpcGoInteropService) StreamingOutputCall(
	req *grpc_testing.StreamingOutputCallRequest,
	stream grpc_testing.TestService_StreamingOutputCallServer,
) error {
	count := len(req.GetResponseParameters())
	if count == 0 {
		count = 2
	}
	for index := 0; index < count; index++ {
		if err := stream.Send(&grpc_testing.StreamingOutputCallResponse{
			Payload: &grpc_testing.Payload{Body: []byte("chunk")},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (grpcGoInteropService) StreamingInputCall(
	stream grpc_testing.TestService_StreamingInputCallServer,
) error {
	var size int32
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&grpc_testing.StreamingInputCallResponse{
				AggregatedPayloadSize: size,
			})
		}
		if err != nil {
			return err
		}
		if payload := req.GetPayload(); payload != nil {
			size += int32(len(payload.GetBody()))
		}
	}
}

func (grpcGoInteropService) FullDuplexCall(
	stream grpc_testing.TestService_FullDuplexCallServer,
) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&grpc_testing.StreamingOutputCallResponse{
			Payload: req.GetPayload(),
		}); err != nil {
			return err
		}
	}
}

func TestInteropGRPCGoClientToYGGServer(t *testing.T) {
	healthDisabled := false
	srv := startYGGServer(t, Settings{
		Server: ServerConfig{
			Network:       "tcp",
			Address:       "127.0.0.1:0",
			HealthEnabled: &healthDisabled,
		},
	}, handleYGGInterop, nil)
	conn := dialGRPCGo(
		t,
		srv.Info().Address,
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
		ggrpc.WithUserAgent("grpc-go-interop/1.0"),
	)
	client := grpc_testing.NewTestServiceClient(conn)

	t.Run("unary metadata gzip and peer", func(t *testing.T) {
		ctx := gmetadata.AppendToOutgoingContext(
			context.Background(),
			"multi", "one",
			"multi", "two",
			"trace-bin", string([]byte{0, 1, 2, 3}),
		)
		var header, trailer gmetadata.MD
		resp, err := client.UnaryCall(
			ctx,
			&grpc_testing.SimpleRequest{},
			ggrpc.UseCompressor(gzip.Name),
			ggrpc.Header(&header),
			ggrpc.Trailer(&trailer),
		)
		require.NoError(t, err)
		assert.Equal(t, "one,two", resp.GetUsername())
		assert.NotEmpty(t, resp.GetServerId())
		assert.Equal(t, []string{"one", "two"}, header.Get("x-response"))
		assert.Equal(t, []string{"one,two"}, header.Get("x-request-multi"))
		assert.Equal(t, []string{string([]byte{0, 1, 2, 3})}, header.Get("x-request-bin"))
		require.Len(t, header.Get("x-user-agent"), 1)
		assert.Contains(t, header.Get("x-user-agent")[0], "grpc-go-interop/1.0")
		assert.Equal(t, []string{"done"}, trailer.Get("x-trailer"))
	})

	t.Run("server streaming", func(t *testing.T) {
		stream, err := client.StreamingOutputCall(
			context.Background(),
			&grpc_testing.StreamingOutputCallRequest{
				ResponseParameters: []*grpc_testing.ResponseParameters{{}, {}},
			},
		)
		require.NoError(t, err)
		for range 2 {
			resp, recvErr := stream.Recv()
			require.NoError(t, recvErr)
			assert.Equal(t, []byte("chunk"), resp.GetPayload().GetBody())
		}
		_, err = stream.Recv()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("client streaming", func(t *testing.T) {
		stream, err := client.StreamingInputCall(context.Background())
		require.NoError(t, err)
		require.NoError(t, stream.Send(&grpc_testing.StreamingInputCallRequest{
			Payload: &grpc_testing.Payload{Body: []byte("abc")},
		}))
		require.NoError(t, stream.Send(&grpc_testing.StreamingInputCallRequest{
			Payload: &grpc_testing.Payload{Body: []byte("12345")},
		}))
		resp, err := stream.CloseAndRecv()
		require.NoError(t, err)
		assert.Equal(t, int32(8), resp.GetAggregatedPayloadSize())
	})

	t.Run("bidirectional streaming", func(t *testing.T) {
		stream, err := client.FullDuplexCall(context.Background())
		require.NoError(t, err)
		for _, body := range []string{"first", "second"} {
			require.NoError(t, stream.Send(&grpc_testing.StreamingOutputCallRequest{
				Payload: &grpc_testing.Payload{Body: []byte(body)},
			}))
			resp, recvErr := stream.Recv()
			require.NoError(t, recvErr)
			assert.Equal(t, body, string(resp.GetPayload().GetBody()))
		}
		require.NoError(t, stream.CloseSend())
		_, err = stream.Recv()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("status details and trailers", func(t *testing.T) {
		var trailer gmetadata.MD
		_, err := client.UnimplementedCall(
			context.Background(),
			&grpc_testing.Empty{},
			ggrpc.Trailer(&trailer),
		)
		require.Error(t, err)
		st := gstatus.Convert(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Equal(t, []string{"present"}, trailer.Get("x-error-trailer"))
		require.Len(t, st.Details(), 1)
		info, ok := st.Details()[0].(*errdetails.ErrorInfo)
		require.True(t, ok)
		assert.Equal(t, "INTEROP_FAILURE", info.GetReason())
		assert.Equal(t, "yggdrasil", info.GetMetadata()["source"])
	})

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := conn.Invoke(
			ctx,
			interopSlowMethod,
			&grpc_testing.Empty{},
			&grpc_testing.Empty{},
		)
		assert.Equal(t, codes.DeadlineExceeded, gstatus.Code(err))
	})
}

func TestInteropYGGClientToGRPCGoServer(t *testing.T) {
	address := startGRPCGoServer(t, grpcGoInteropService{})
	client := newYGGClientWithConfig(t, address, ClientConfig{
		Compressor: gzip.Name,
		Transport: ClientTransportOptions{
			UserAgent: "yggdrasil-interop/1.0",
		},
	}, nil)

	t.Run("unary metadata gzip and status", func(t *testing.T) {
		ctx := ymetadata.WithStreamContext(ymetadata.WithOutContext(
			context.Background(),
			ymetadata.Pairs(
				"multi", "one",
				"multi", "two",
				"trace-bin", string([]byte{0, 1, 2, 3}),
			),
		))
		st, err := client.NewStream(
			ctx,
			&stream.Desc{},
			grpc_testing.TestService_UnaryCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, st.SendMsg(&grpc_testing.SimpleRequest{}))
		resp := new(grpc_testing.SimpleResponse)
		require.NoError(t, st.RecvMsg(resp))
		assert.Equal(t, "one,two", resp.GetUsername())
		assert.NotEmpty(t, resp.GetServerId())
		header, err := st.Header()
		require.NoError(t, err)
		assert.Equal(t, []string{"one", "two"}, header.Get("x-response"))
		assert.Equal(t, []string{string([]byte{0, 1, 2, 3})}, header.Get("x-request-bin"))
		require.Len(t, header.Get("x-user-agent"), 1)
		assert.Contains(t, header.Get("x-user-agent")[0], "yggdrasil-interop/1.0")
		trailer := st.Trailer()
		assert.Equal(t, []string{"done"}, trailer.Get("x-trailer"))
	})

	t.Run("status details", func(t *testing.T) {
		st, err := client.NewStream(
			ymetadata.WithStreamContext(context.Background()),
			&stream.Desc{},
			grpc_testing.TestService_UnaryCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, st.SendMsg(&grpc_testing.SimpleRequest{FillOauthScope: true}))
		err = st.RecvMsg(new(grpc_testing.SimpleResponse))
		require.Error(t, err)
		status, ok := ystatus.CoverError(err)
		require.True(t, ok)
		assert.Equal(t, code.Code_INVALID_ARGUMENT, status.Code())
		info := status.ErrorInfo()
		require.NotNil(t, info)
		assert.Equal(t, "GRPC_GO_FAILURE", info.GetReason())
		assert.Equal(t, "grpc-go", info.GetMetadata()["source"])
	})

	t.Run("server streaming", func(t *testing.T) {
		st, err := client.NewStream(
			context.Background(),
			&stream.Desc{ServerStreams: true},
			grpc_testing.TestService_StreamingOutputCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, st.SendMsg(&grpc_testing.StreamingOutputCallRequest{
			ResponseParameters: []*grpc_testing.ResponseParameters{{}, {}},
		}))
		require.NoError(t, st.CloseSend())
		for range 2 {
			resp := new(grpc_testing.StreamingOutputCallResponse)
			require.NoError(t, st.RecvMsg(resp))
			assert.Equal(t, []byte("chunk"), resp.GetPayload().GetBody())
		}
		assert.ErrorIs(t, st.RecvMsg(new(grpc_testing.StreamingOutputCallResponse)), io.EOF)
	})

	t.Run("client streaming", func(t *testing.T) {
		st, err := client.NewStream(
			context.Background(),
			&stream.Desc{ClientStreams: true},
			grpc_testing.TestService_StreamingInputCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, st.SendMsg(&grpc_testing.StreamingInputCallRequest{
			Payload: &grpc_testing.Payload{Body: []byte("abc")},
		}))
		require.NoError(t, st.SendMsg(&grpc_testing.StreamingInputCallRequest{
			Payload: &grpc_testing.Payload{Body: []byte("12345")},
		}))
		require.NoError(t, st.CloseSend())
		resp := new(grpc_testing.StreamingInputCallResponse)
		require.NoError(t, st.RecvMsg(resp))
		assert.Equal(t, int32(8), resp.GetAggregatedPayloadSize())
	})

	t.Run("bidirectional streaming", func(t *testing.T) {
		st, err := client.NewStream(
			context.Background(),
			&stream.Desc{ClientStreams: true, ServerStreams: true},
			grpc_testing.TestService_FullDuplexCall_FullMethodName,
		)
		require.NoError(t, err)
		for _, body := range []string{"first", "second"} {
			require.NoError(t, st.SendMsg(&grpc_testing.StreamingOutputCallRequest{
				Payload: &grpc_testing.Payload{Body: []byte(body)},
			}))
			resp := new(grpc_testing.StreamingOutputCallResponse)
			require.NoError(t, st.RecvMsg(resp))
			assert.Equal(t, body, string(resp.GetPayload().GetBody()))
		}
		require.NoError(t, st.CloseSend())
		assert.ErrorIs(t, st.RecvMsg(new(grpc_testing.StreamingOutputCallResponse)), io.EOF)
	})

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		st, err := client.NewStream(
			ctx,
			&stream.Desc{},
			grpc_testing.TestService_UnaryCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, st.SendMsg(&grpc_testing.SimpleRequest{FillServerId: true}))
		err = st.RecvMsg(new(grpc_testing.SimpleResponse))
		require.Error(t, err)
		status, ok := ystatus.CoverError(err)
		require.True(t, ok)
		assert.Equal(t, code.Code_DEADLINE_EXCEEDED, status.Code())
	})
}

func TestInteropCancellation(t *testing.T) {
	t.Run("grpc-go client to yggdrasil server", func(t *testing.T) {
		healthDisabled := false
		srv := startYGGServer(t, Settings{
			Server: ServerConfig{
				Network:       "tcp",
				Address:       "127.0.0.1:0",
				HealthEnabled: &healthDisabled,
			},
		}, handleYGGInterop, nil)
		conn := dialGRPCGo(t, srv.Info().Address)

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			result <- conn.Invoke(
				ctx,
				interopSlowMethod,
				&grpc_testing.Empty{},
				&grpc_testing.Empty{},
			)
		}()
		time.AfterFunc(20*time.Millisecond, cancel)

		select {
		case err := <-result:
			assert.Equal(t, codes.Canceled, gstatus.Code(err))
		case <-time.After(2 * time.Second):
			t.Fatal("canceled grpc-go call did not return")
		}
	})

	t.Run("yggdrasil client to grpc-go server", func(t *testing.T) {
		address := startGRPCGoServer(t, grpcGoInteropService{})
		client := newYGGClient(t, address, "", nil)

		ctx, cancel := context.WithCancel(ymetadata.WithStreamContext(context.Background()))
		call, err := client.NewStream(
			ctx,
			&stream.Desc{},
			grpc_testing.TestService_UnaryCall_FullMethodName,
		)
		require.NoError(t, err)
		require.NoError(t, call.SendMsg(&grpc_testing.SimpleRequest{FillServerId: true}))

		result := make(chan error, 1)
		go func() {
			result <- call.RecvMsg(new(grpc_testing.SimpleResponse))
		}()
		time.AfterFunc(20*time.Millisecond, cancel)

		select {
		case err := <-result:
			require.Error(t, err)
			status, ok := ystatus.CoverError(err)
			require.True(t, ok)
			assert.Equal(t, code.Code_CANCELLED, status.Code())
		case <-time.After(2 * time.Second):
			t.Fatal("canceled yggdrasil call did not return")
		}
	})
}
