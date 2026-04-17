// Package gzip implements and registers the gzip compressor
// during the initialization.
package gzip

import grpcgzip "google.golang.org/grpc/encoding/gzip"

// Name is the name registered for the gzip compressor.
const Name = grpcgzip.Name

// SetLevel updates the registered gzip compressor to use the specified level.
func SetLevel(level int) error {
	return grpcgzip.SetLevel(level)
}
