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

package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/codesjoy/yggdrasil/v3/transport/support/marshaler"
)

// PopulateQueryParameters parses query parameters
// into "msg" using current query parser
func PopulateQueryParameters(msg proto.Message, values url.Values) error {
	return PopulateQueryParametersContext(context.Background(), msg, values)
}

// PopulateQueryParametersContext parses query parameters into msg while excluding
// fields whose canonical Protobuf paths start with one of excludedFieldPaths.
func PopulateQueryParametersContext(
	ctx context.Context,
	msg proto.Message,
	values url.Values,
	excludedFieldPaths ...string,
) error {
	decoder := queryParameterMarshaler(ctx)
	msgValue := msg.ProtoReflect()
	excludedPaths := normalizeExcludedFieldPaths(msgValue, excludedFieldPaths)
	for key, value := range values {
		fieldPath := normalizeFieldPath(msgValue, strings.Split(key, "."))
		if hasExcludedFieldPathPrefix(fieldPath, excludedPaths) {
			continue
		}
		if err := populateFieldValues(msgValue, fieldPath, value, decoder); err != nil {
			return err
		}
	}
	return nil
}

// PopulateFieldFromPath sets a value in a nested Protobuf structure.
func PopulateFieldFromPath(msg proto.Message, fieldPathString string, value string) error {
	return PopulateFieldFromPathContext(context.Background(), msg, fieldPathString, value)
}

// PopulateFieldFromPathContext sets a value in a nested Protobuf structure.
func PopulateFieldFromPathContext(
	ctx context.Context,
	msg proto.Message,
	fieldPathString string,
	value string,
) error {
	fieldPath := strings.Split(fieldPathString, ".")
	return populateFieldValues(
		msg.ProtoReflect(),
		fieldPath,
		[]string{value},
		queryParameterMarshaler(ctx),
	)
}

func queryParameterMarshaler(ctx context.Context) marshaler.Marshaler {
	inbound := marshaler.InboundFromContext(ctx)
	if jsonpb, ok := inbound.(*marshaler.JSONPb); ok {
		return jsonpb
	}

	_, cfg := currentMarshalerConfig()
	return marshaler.NewJSONPbMarshalerWithConfig(cfg)
}

func normalizeExcludedFieldPaths(
	msgValue protoreflect.Message,
	excludedFieldPaths []string,
) [][]string {
	paths := make([][]string, 0, len(excludedFieldPaths))
	for _, fieldPath := range excludedFieldPaths {
		if fieldPath == "" {
			continue
		}
		paths = append(paths, normalizeFieldPath(msgValue, strings.Split(fieldPath, ".")))
	}
	return paths
}

func normalizeFieldPath(msgValue protoreflect.Message, fieldPath []string) []string {
	normalized := make([]string, 0, len(fieldPath))
	for i, fieldName := range fieldPath {
		fields := msgValue.Descriptor().Fields()
		fd := fields.ByName(protoreflect.Name(fieldName))
		if fd == nil {
			fd = fields.ByJSONName(fieldName)
		}
		if fd == nil {
			return append(normalized, fieldPath[i:]...)
		}

		normalized = append(normalized, string(fd.Name()))
		if i == len(fieldPath)-1 {
			break
		}
		if fd.Message() == nil || fd.Cardinality() == protoreflect.Repeated {
			return append(normalized, fieldPath[i+1:]...)
		}
		msgValue = msgValue.Get(fd).Message()
	}
	return normalized
}

func hasExcludedFieldPathPrefix(fieldPath []string, excludedPaths [][]string) bool {
	for _, excludedPath := range excludedPaths {
		if len(excludedPath) > len(fieldPath) {
			continue
		}
		matched := true
		for i := range excludedPath {
			if excludedPath[i] != fieldPath[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func populateFieldValues(
	v protoreflect.Message,
	fieldPath []string,
	values []string,
	decoder marshaler.Marshaler,
) error {
	if len(fieldPath) < 1 {
		return errors.New("no field path")
	}
	if len(values) < 1 {
		return errors.New("no value provided")
	}
	var fd protoreflect.FieldDescriptor
	for i, fieldName := range fieldPath {
		fields := v.Descriptor().Fields()

		if fd = fields.ByName(protoreflect.Name(fieldName)); fd == nil {
			fd = fields.ByJSONName(fieldName)
			if fd == nil {
				slog.Info(
					"field not found",
					"field",
					fieldName,
					"message",
					v.Descriptor().FullName(),
				)
				return nil
			}
		}

		if i == len(fieldPath)-1 {
			break
		}

		if fd.Message() == nil || fd.Cardinality() == protoreflect.Repeated {
			return fmt.Errorf("invalid path: %q is not a message", fieldName)
		}

		v = v.Mutable(fd).Message()
	}
	if of := fd.ContainingOneof(); of != nil {
		if f := v.WhichOneof(of); f != nil {
			return fmt.Errorf("field already set for oneof %q", of.FullName().Name())
		}
	}
	switch {
	case fd.IsList():
		return populateRepeatedField(fd, v.Mutable(fd).List(), values, decoder)
	case fd.IsMap():
		return populateMapField(fd, v.Mutable(fd).Map(), values, decoder)
	}
	if len(values) != 1 {
		return fmt.Errorf(
			"too many values for field %q: %s",
			fd.FullName().Name(),
			strings.Join(values, ", "),
		)
	}
	return populateField(fd, v, values[0], decoder)
}

func populateField(
	fd protoreflect.FieldDescriptor,
	v protoreflect.Message,
	value string,
	decoder marshaler.Marshaler,
) error {
	val, err := parseField(fd, value, decoder)
	if err != nil {
		return fmt.Errorf("parsing field %q: %w", fd.FullName().Name(), err)
	}
	v.Set(fd, val)
	return nil
}

func populateRepeatedField(
	fd protoreflect.FieldDescriptor,
	list protoreflect.List,
	values []string,
	decoder marshaler.Marshaler,
) error {
	for _, value := range values {
		v, err := parseField(fd, value, decoder)
		if err != nil {
			return fmt.Errorf("parsing list %q: %w", fd.FullName().Name(), err)
		}
		list.Append(v)
	}
	return nil
}

func populateMapField(
	fd protoreflect.FieldDescriptor,
	mp protoreflect.Map,
	values []string,
	decoder marshaler.Marshaler,
) error {
	if len(values) != 2 {
		return fmt.Errorf(
			"more than one value provided for key %q in map %q",
			values[0],
			fd.FullName(),
		)
	}
	key, err := parseField(fd.MapKey(), values[0], decoder)
	if err != nil {
		return fmt.Errorf("parsing map key %q: %w", fd.FullName().Name(), err)
	}
	value, err := parseField(fd.MapValue(), values[1], decoder)
	if err != nil {
		return fmt.Errorf("parsing map value %q: %w", fd.FullName().Name(), err)
	}
	mp.Set(key.MapKey(), value)
	return nil
}

func parseField(
	fd protoreflect.FieldDescriptor,
	value string,
	decoder marshaler.Marshaler,
) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(v), nil
	case protoreflect.EnumKind:
		enum, err := protoregistry.GlobalTypes.FindEnumByName(fd.Enum().FullName())
		switch {
		case errors.Is(err, protoregistry.NotFound):
			return protoreflect.Value{}, fmt.Errorf(
				"enum %q is not registered",
				fd.Enum().FullName(),
			)
		case err != nil:
			return protoreflect.Value{}, fmt.Errorf("failed to look up enum: %w", err)
		}
		v := enum.Descriptor().Values().ByName(protoreflect.Name(value))
		if v == nil {
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return protoreflect.Value{}, fmt.Errorf("%q is not a valid value", value)
			}
			v = enum.Descriptor().Values().ByNumber(protoreflect.EnumNumber(i))
			if v == nil {
				return protoreflect.Value{}, fmt.Errorf("%q is not a valid value", value)
			}
		}
		return protoreflect.ValueOfEnum(v.Number()), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		v, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(v)), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(v), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(v)), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(v), nil
	case protoreflect.FloatKind:
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat32(float32(v)), nil
	case protoreflect.DoubleKind:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfFloat64(v), nil
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(value), nil
	case protoreflect.BytesKind:
		v, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBytes(v), nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return parseMessage(fd.Message(), value, decoder)
	default:
		return protoreflect.Value{}, fmt.Errorf("unknown field kind: %v", fd.Kind())
	}
}

func parseMessage(
	md protoreflect.MessageDescriptor,
	value string,
	decoder marshaler.Marshaler,
) (protoreflect.Value, error) {
	var msg proto.Message
	switch md.FullName() {
	case "google.protobuf.Timestamp":
		if value == "null" {
			break
		}
		t, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = timestamppb.New(t)
	case "google.protobuf.Duration":
		if value == "null" {
			break
		}
		d, err := time.ParseDuration(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = durationpb.New(d)
	case "google.protobuf.DoubleValue":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Double(v)
	case "google.protobuf.FloatValue":
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Float(float32(v))
	case "google.protobuf.Int64Value":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Int64(v)
	case "google.protobuf.Int32Value":
		v, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Int32(int32(v))
	case "google.protobuf.UInt64Value":
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.UInt64(v)
	case "google.protobuf.UInt32Value":
		v, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.UInt32(uint32(v))
	case "google.protobuf.BoolValue":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Bool(v)
	case "google.protobuf.StringValue":
		msg = wrapperspb.String(value)
	case "google.protobuf.BytesValue":
		v, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		msg = wrapperspb.Bytes(v)
	case "google.protobuf.FieldMask":
		fm := &fieldmaskpb.FieldMask{}
		value = strings.TrimSpace(value)
		if value == "*" {
			fm.Paths = []string{"*"}
			msg = fm
			break
		}
		for _, path := range strings.Split(value, ",") {
			if path == "*" {
				return protoreflect.Value{}, errors.New(
					"field mask wildcard must be used alone",
				)
			}
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return protoreflect.Value{}, err
		}
		if err := decoder.Unmarshal(encoded, fm); err != nil {
			return protoreflect.Value{}, err
		}
		msg = fm
	default:
		return protoreflect.Value{}, fmt.Errorf(
			"unsupported message type: %q",
			string(md.FullName()),
		)
	}
	return protoreflect.ValueOfMessage(msg.ProtoReflect()), nil
}
