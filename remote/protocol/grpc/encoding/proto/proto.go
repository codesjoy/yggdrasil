// Package proto defines the protobuf codec. Importing this package will
// register the codec.
package proto

import grpcproto "google.golang.org/grpc/encoding/proto"

// Name is the name registered for the proto compressor.
const Name = grpcproto.Name
