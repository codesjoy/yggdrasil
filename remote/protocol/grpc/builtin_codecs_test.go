package grpc

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stpb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/codesjoy/yggdrasil/v2/remote/protocol/grpc/encoding"
)

func TestBuiltinCodecsAndCompressorAvailableWithoutExtraImports(t *testing.T) {
	protoCodec := encoding.GetCodec("proto")
	require.NotNil(t, protoCodec)

	rawCodec := encoding.GetCodec("raw")
	require.NotNil(t, rawCodec)

	jsonRawCodec := encoding.GetCodec("jsonraw")
	require.NotNil(t, jsonRawCodec)

	compressor := encoding.GetCompressor("gzip")
	require.NotNil(t, compressor)

	msg := &stpb.Status{Message: "hello"}
	wire, err := protoCodec.Marshal(msg)
	require.NoError(t, err)
	var decoded stpb.Status
	require.NoError(t, protoCodec.Unmarshal(wire, &decoded))
	assert.Equal(t, msg.Message, decoded.Message)

	rawPayload := []byte("raw-payload")
	rawWire, err := rawCodec.Marshal(rawPayload)
	require.NoError(t, err)
	var rawDecoded []byte
	require.NoError(t, rawCodec.Unmarshal(rawWire, &rawDecoded))
	assert.Equal(t, rawPayload, rawDecoded)

	jsonPayload := []byte(`{"message":"jsonraw-payload"}`)
	jsonWire, err := jsonRawCodec.Marshal(jsonPayload)
	require.NoError(t, err)
	var jsonDecoded []byte
	require.NoError(t, jsonRawCodec.Unmarshal(jsonWire, &jsonDecoded))
	assert.Equal(t, jsonPayload, jsonDecoded)

	var compressed bytes.Buffer
	writer, err := compressor.Compress(&compressed)
	require.NoError(t, err)
	_, err = writer.Write([]byte("gzip-payload"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	reader, err := compressor.Decompress(bytes.NewReader(compressed.Bytes()))
	require.NoError(t, err)
	var roundTrip bytes.Buffer
	_, err = roundTrip.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, "gzip-payload", roundTrip.String())
}
