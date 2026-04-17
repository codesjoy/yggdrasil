package grpc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/codesjoy/pkg/basic/xerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/code"
	stpb "google.golang.org/genproto/googleapis/rpc/status"
	ggrpc "google.golang.org/grpc"
	gcodes "google.golang.org/grpc/codes"
	ginsecure "google.golang.org/grpc/credentials/insecure"
	grpcencoding "google.golang.org/grpc/encoding"
	gmetadata "google.golang.org/grpc/metadata"
	gstatus "google.golang.org/grpc/status"

	"github.com/codesjoy/yggdrasil/v2/config"
	ymetadata "github.com/codesjoy/yggdrasil/v2/metadata"
	"github.com/codesjoy/yggdrasil/v2/remote"
	jsonrawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/jsonraw"
	rawenc "github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding/raw"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	"github.com/codesjoy/yggdrasil/v2/stats"
	yggstatus "github.com/codesjoy/yggdrasil/v2/status"
	"github.com/codesjoy/yggdrasil/v2/stream"
)

const (
	protoUnaryMethod       = "/proto.Test/Unary"
	protoUnaryStatusMethod = "/proto.Test/UnaryStatus"
	protoBidiMethod        = "/proto.Test/Bidi"
)

type grpcGoProtoInteropServer interface {
	Unary(context.Context, *stpb.Status) (*stpb.Status, error)
	UnaryStatus(context.Context, *stpb.Status) (*stpb.Status, error)
	Bidi(ggrpc.ServerStream) error
}

type grpcGoProtoInteropService struct {
	unaryFunc       func(context.Context, *stpb.Status) (*stpb.Status, error)
	unaryStatusFunc func(context.Context, *stpb.Status) (*stpb.Status, error)
	bidiFunc        func(ggrpc.ServerStream) error
}

func (s grpcGoProtoInteropService) Unary(ctx context.Context, in *stpb.Status) (*stpb.Status, error) {
	if s.unaryFunc == nil {
		return nil, gstatus.Error(gcodes.Unimplemented, "unary not implemented")
	}
	return s.unaryFunc(ctx, in)
}

func (s grpcGoProtoInteropService) UnaryStatus(ctx context.Context, in *stpb.Status) (*stpb.Status, error) {
	if s.unaryStatusFunc == nil {
		return nil, gstatus.Error(gcodes.Unimplemented, "unary status not implemented")
	}
	return s.unaryStatusFunc(ctx, in)
}

func (s grpcGoProtoInteropService) Bidi(stream ggrpc.ServerStream) error {
	if s.bidiFunc == nil {
		return gstatus.Error(gcodes.Unimplemented, "bidi not implemented")
	}
	return s.bidiFunc(stream)
}

type grpcJSONRawInteropService struct{}

type grpcJSONRawServer interface{}

func startInteropYggServer(t *testing.T, handle remote.MethodHandle) (*server, chan error) {
	t.Helper()

	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp"))
	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "address"), "127.0.0.1:0"))

	srv, err := newServer(handle)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Handle()
	}()
	return srv.(*server), serveErrCh
}

func newInteropYggClient(t *testing.T, serviceName, addr, compressor string) *clientConn {
	t.Helper()

	require.NoError(t, config.Set(config.Join(config.KeyBase, "remote", "protocol", scheme, "network"), "tcp"))
	require.NoError(
		t,
		config.Set(
			config.Join(config.KeyBase, "client", "{"+serviceName+"}", "protocol_config", scheme, "network"),
			"tcp",
		),
	)
	require.NoError(
		t,
		config.Set(
			config.Join(config.KeyBase, "client", "{"+serviceName+"}", "protocol_config", scheme, "compressor"),
			compressor,
		),
	)

	cli, err := newClient(
		context.Background(),
		serviceName,
		resolver.BaseEndpoint{Address: addr, Protocol: scheme},
		stats.NoOpHandler,
		func(remote.ClientState) {},
	)
	require.NoError(t, err)
	cc := cli.(*clientConn)
	cc.Connect()
	require.Eventually(t, func() bool {
		return cc.State() == remote.Ready
	}, 5*time.Second, 20*time.Millisecond)
	return cc
}

func startGrpcGoProtoInteropServer(
	t *testing.T,
	service grpcGoProtoInteropServer,
	opts ...ggrpc.ServerOption,
) (net.Listener, *ggrpc.Server, chan error) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	gs := ggrpc.NewServer(opts...)
	gs.RegisterService(&ggrpc.ServiceDesc{
		ServiceName: "proto.Test",
		HandlerType: (*grpcGoProtoInteropServer)(nil),
		Methods: []ggrpc.MethodDesc{
			{
				MethodName: "Unary",
				Handler: func(
					srv interface{},
					ctx context.Context,
					dec func(interface{}) error,
					interceptor ggrpc.UnaryServerInterceptor,
				) (interface{}, error) {
					in := new(stpb.Status)
					if err := dec(in); err != nil {
						return nil, err
					}
					impl := srv.(grpcGoProtoInteropServer)
					if interceptor == nil {
						return impl.Unary(ctx, in)
					}
					info := &ggrpc.UnaryServerInfo{
						Server:     impl,
						FullMethod: protoUnaryMethod,
					}
					handler := func(ctx context.Context, req interface{}) (interface{}, error) {
						return impl.Unary(ctx, req.(*stpb.Status))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "UnaryStatus",
				Handler: func(
					srv interface{},
					ctx context.Context,
					dec func(interface{}) error,
					interceptor ggrpc.UnaryServerInterceptor,
				) (interface{}, error) {
					in := new(stpb.Status)
					if err := dec(in); err != nil {
						return nil, err
					}
					impl := srv.(grpcGoProtoInteropServer)
					if interceptor == nil {
						return impl.UnaryStatus(ctx, in)
					}
					info := &ggrpc.UnaryServerInfo{
						Server:     impl,
						FullMethod: protoUnaryStatusMethod,
					}
					handler := func(ctx context.Context, req interface{}) (interface{}, error) {
						return impl.UnaryStatus(ctx, req.(*stpb.Status))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams: []ggrpc.StreamDesc{{
			StreamName: "Bidi",
			Handler: func(srv interface{}, stream ggrpc.ServerStream) error {
				return srv.(grpcGoProtoInteropServer).Bidi(stream)
			},
			ServerStreams: true,
			ClientStreams: true,
		}},
	}, service)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- gs.Serve(lis)
	}()
	return lis, gs, serveErrCh
}

func requireYggStatus(t *testing.T, err error, wantCode code.Code, wantMessage string) {
	t.Helper()

	st, ok := yggstatus.CoverError(err)
	require.True(t, ok, "error does not wrap yggdrasil status: %v", err)
	require.Equal(t, wantCode, st.Code())
	require.Equal(t, wantMessage, st.Message())
}

func mustGetGRPCCodecV2(t *testing.T, name string) grpcencoding.CodecV2 {
	t.Helper()

	codec := grpcencoding.GetCodecV2(name)
	require.NotNil(t, codec)
	return codec
}

func TestInterop_GrpcGoClient_ToYggServer_ProtoUnaryWithGzipMetadata(t *testing.T) {
	srv, serveErrCh := startInteropYggServer(t, func(ss remote.ServerStream) {
		var (
			reply any
			err   error
		)
		defer func() { ss.Finish(reply, err) }()

		require.NoError(t, ss.Start(false, false))

		inMD, ok := ymetadata.FromInContext(ss.Context())
		require.True(t, ok)
		require.Equal(t, []string{"from-grpc"}, inMD.Get("client-header"))

		require.NoError(t, ymetadata.SetHeader(ss.Context(), ymetadata.Pairs("server-header", "from-ygg")))
		require.NoError(t, ymetadata.SetTrailer(ss.Context(), ymetadata.Pairs("server-trailer", "done")))

		var req stpb.Status
		require.NoError(t, ss.RecvMsg(&req))
		reply = &stpb.Status{Message: req.Message + ":ok"}
	})
	defer func() {
		require.NoError(t, srv.Stop(context.Background()))
		require.NoError(t, <-serveErrCh)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = gmetadata.AppendToOutgoingContext(ctx, "client-header", "from-grpc")

	conn, err := ggrpc.NewClient(
		srv.Info().Address,
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
		ggrpc.WithDefaultCallOptions(ggrpc.UseCompressor("gzip")),
	)
	require.NoError(t, err)
	defer conn.Close()
	conn.Connect()

	var (
		out     stpb.Status
		header  gmetadata.MD
		trailer gmetadata.MD
	)
	err = conn.Invoke(
		ctx,
		protoUnaryMethod,
		&stpb.Status{Message: "hello"},
		&out,
		ggrpc.Header(&header),
		ggrpc.Trailer(&trailer),
	)
	require.NoError(t, err)
	assert.Equal(t, "hello:ok", out.Message)
	assert.Equal(t, []string{"from-ygg"}, header.Get("server-header"))
	assert.Equal(t, []string{"done"}, trailer.Get("server-trailer"))
}

func TestInterop_GrpcGoClient_ToYggServer_ProtoStatusMapping(t *testing.T) {
	srv, serveErrCh := startInteropYggServer(t, func(ss remote.ServerStream) {
		var err error
		defer func() { ss.Finish(nil, err) }()

		require.NoError(t, ss.Start(false, false))
		var req stpb.Status
		require.NoError(t, ss.RecvMsg(&req))
		err = xerror.New(code.Code_INVALID_ARGUMENT, "bad request")
	})
	defer func() {
		require.NoError(t, srv.Stop(context.Background()))
		require.NoError(t, <-serveErrCh)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := ggrpc.NewClient(
		srv.Info().Address,
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()
	conn.Connect()

	var out stpb.Status
	err = conn.Invoke(ctx, protoUnaryMethod, &stpb.Status{Message: "hello"}, &out)
	require.Error(t, err)
	st, ok := gstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, gcodes.InvalidArgument, st.Code())
	assert.Equal(t, "bad request", st.Message())
}

func TestInterop_GrpcGoClient_ToYggServer_ProtoBidiStream(t *testing.T) {
	srv, serveErrCh := startInteropYggServer(t, func(ss remote.ServerStream) {
		var err error
		defer func() { ss.Finish(nil, err) }()

		require.NoError(t, ss.Start(true, true))
		for {
			var req stpb.Status
			err = ss.RecvMsg(&req)
			if err == io.EOF {
				err = nil
				return
			}
			require.NoError(t, err)
			require.NoError(t, ss.SendMsg(&stpb.Status{Message: req.Message + ":echo"}))
		}
	})
	defer func() {
		require.NoError(t, srv.Stop(context.Background()))
		require.NoError(t, <-serveErrCh)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := ggrpc.NewClient(
		srv.Info().Address,
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()
	conn.Connect()

	cs, err := conn.NewStream(
		ctx,
		&ggrpc.StreamDesc{ServerStreams: true, ClientStreams: true},
		protoBidiMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "a"}))
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "b"}))

	var out stpb.Status
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, "a:echo", out.Message)
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, "b:echo", out.Message)
	require.NoError(t, cs.CloseSend())
	assert.ErrorIs(t, cs.RecvMsg(&out), io.EOF)
}

func TestInterop_YggClient_ToGrpcGoServer_ProtoUnaryWithGzipMetadata(t *testing.T) {
	lis, gs, serveErrCh := startGrpcGoProtoInteropServer(t, grpcGoProtoInteropService{
		unaryFunc: func(ctx context.Context, in *stpb.Status) (*stpb.Status, error) {
			md, ok := gmetadata.FromIncomingContext(ctx)
			require.True(t, ok)
			require.Equal(t, []string{"from-ygg"}, md.Get("client-header"))

			require.NoError(t, ggrpc.SetHeader(ctx, gmetadata.Pairs("server-header", "from-grpc")))
			require.NoError(t, ggrpc.SetTrailer(ctx, gmetadata.Pairs("server-trailer", "done")))
			return &stpb.Status{Message: in.Message + ":ok"}, nil
		},
	})
	defer func() {
		gs.GracefulStop()
		require.NoError(t, <-serveErrCh)
	}()

	cc := newInteropYggClient(t, "grpc-interop-proto-metadata", lis.Addr().String(), "gzip")
	defer func() {
		require.NoError(t, cc.Close())
	}()

	ctx := ymetadata.WithOutContext(context.Background(), ymetadata.Pairs("client-header", "from-ygg"))
	cs, err := cc.NewStream(ctx, &stream.Desc{}, protoUnaryMethod)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "hello"}))

	header, err := cs.Header()
	require.NoError(t, err)
	assert.Equal(t, []string{"from-grpc"}, header.Get("server-header"))

	var out stpb.Status
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, "hello:ok", out.Message)
	assert.Equal(t, []string{"done"}, cs.Trailer().Get("server-trailer"))
}

func TestInterop_YggClient_ToGrpcGoServer_ProtoStatusMapping(t *testing.T) {
	lis, gs, serveErrCh := startGrpcGoProtoInteropServer(t, grpcGoProtoInteropService{
		unaryStatusFunc: func(context.Context, *stpb.Status) (*stpb.Status, error) {
			return nil, gstatus.Error(gcodes.InvalidArgument, "bad request")
		},
	})
	defer func() {
		gs.GracefulStop()
		require.NoError(t, <-serveErrCh)
	}()

	cc := newInteropYggClient(t, "grpc-interop-proto-status", lis.Addr().String(), "")
	defer func() {
		require.NoError(t, cc.Close())
	}()

	cs, err := cc.NewStream(context.Background(), &stream.Desc{}, protoUnaryStatusMethod)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "hello"}))

	var out stpb.Status
	err = cs.RecvMsg(&out)
	require.Error(t, err)
	requireYggStatus(t, err, code.Code_INVALID_ARGUMENT, "bad request")
}

func TestInterop_YggClient_ToGrpcGoServer_ProtoBidiStream(t *testing.T) {
	lis, gs, serveErrCh := startGrpcGoProtoInteropServer(t, grpcGoProtoInteropService{
		bidiFunc: func(stream ggrpc.ServerStream) error {
			for {
				var in stpb.Status
				err := stream.RecvMsg(&in)
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				if err := stream.SendMsg(&stpb.Status{Message: in.Message + ":echo"}); err != nil {
					return err
				}
			}
		},
	})
	defer func() {
		gs.GracefulStop()
		require.NoError(t, <-serveErrCh)
	}()

	cc := newInteropYggClient(t, "grpc-interop-proto-bidi", lis.Addr().String(), "")
	defer func() {
		require.NoError(t, cc.Close())
	}()

	cs, err := cc.NewStream(
		context.Background(),
		&stream.Desc{ServerStreams: true, ClientStreams: true},
		protoBidiMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "a"}))
	require.NoError(t, cs.SendMsg(&stpb.Status{Message: "b"}))

	var out stpb.Status
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, "a:echo", out.Message)
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, "b:echo", out.Message)
	require.NoError(t, cs.CloseSend())
	assert.True(t, isSuccessfulStreamEnd(cs.RecvMsg(&out)))
}

func TestInterop_GrpcGoClient_ToYggServer_RawUnary(t *testing.T) {
	srv, serveErrCh := startInteropYggServer(t, rawTestMethodHandle)
	defer func() {
		require.NoError(t, srv.Stop(context.Background()))
		require.NoError(t, <-serveErrCh)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := ggrpc.NewClient(
		srv.Info().Address,
		ggrpc.WithTransportCredentials(ginsecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()
	conn.Connect()

	cs, err := conn.NewStream(
		ctx,
		&ggrpc.StreamDesc{},
		rawUnaryMethod,
		ggrpc.CallContentSubtype(rawenc.Name),
		ggrpc.ForceCodecV2(mustGetGRPCCodecV2(t, rawenc.Name)),
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte("ping")))
	var out []byte
	require.NoError(t, cs.RecvMsg(&out))
	assert.Equal(t, []byte("unary:ping"), out)
}

func TestInterop_YggClient_ToGrpcGoServer_JSONRawUnary(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	gs := ggrpc.NewServer(ggrpc.ForceServerCodecV2(mustGetGRPCCodecV2(t, jsonrawenc.Name)))
	gs.RegisterService(&ggrpc.ServiceDesc{
		ServiceName: "jsonraw.Test",
		HandlerType: (*grpcJSONRawServer)(nil),
		Methods: []ggrpc.MethodDesc{{
			MethodName: "Unary",
			Handler: func(
				srv interface{},
				ctx context.Context,
				dec func(interface{}) error,
				interceptor ggrpc.UnaryServerInterceptor,
			) (interface{}, error) {
				var in []byte
				if err := dec(&in); err != nil {
					return nil, err
				}
				handler := func(context.Context, interface{}) (interface{}, error) {
					return []byte(`{"message":"pong"}`), nil
				}
				if interceptor == nil {
					return handler(ctx, in)
				}
				info := &ggrpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: jsonRawUnaryMethod,
				}
				return interceptor(ctx, in, info, handler)
			},
		}},
	}, grpcJSONRawInteropService{})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- gs.Serve(lis)
	}()
	defer func() {
		gs.GracefulStop()
		require.NoError(t, <-serveErrCh)
	}()

	cc := newInteropYggClient(t, "grpc-interop-jsonraw", lis.Addr().String(), "")
	defer func() {
		require.NoError(t, cc.Close())
	}()

	cs, err := cc.NewStream(
		WithCallOptions(context.Background(), CallContentSubtype(jsonrawenc.Name)),
		&stream.Desc{},
		jsonRawUnaryMethod,
	)
	require.NoError(t, err)
	require.NoError(t, cs.SendMsg([]byte(`{"message":"ping"}`)))
	var out []byte
	require.NoError(t, cs.RecvMsg(&out))
	assert.JSONEq(t, `{"message":"pong"}`, string(out))
}
