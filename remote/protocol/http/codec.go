package http

import (
	"errors"
	"mime"
	"strings"
	"sync"

	"github.com/codesjoy/yggdrasil/v2/remote/marshaler"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	scheme           = "http"
	contentTypeJSON  = "application/json"
	contentTypeProto = "application/octet-stream"
)

func normalizeContentType(v string) string {
	if v == "" {
		return ""
	}
	ct, _, err := mime.ParseMediaType(v)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return strings.ToLower(ct)
}

func buildMarshaler(cfg *MarshalerConfig) (marshaler.Marshaler, error) {
	if cfg == nil {
		return &marshaler.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}, nil
	}

	switch cfg.Type {
	case "jsonpb":
		if cfg.Config == nil {
			return &marshaler.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			}, nil
		}
		s := &marshaler.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				Multiline:       cfg.Config.MarshalOptions.Multiline,
				Indent:          cfg.Config.MarshalOptions.Indent,
				AllowPartial:    cfg.Config.MarshalOptions.AllowPartial,
				UseProtoNames:   cfg.Config.MarshalOptions.UseProtoNames,
				UseEnumNumbers:  cfg.Config.MarshalOptions.UseEnumNumbers,
				EmitUnpopulated: cfg.Config.MarshalOptions.EmitUnpopulated,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				AllowPartial:   cfg.Config.UnmarshalOptions.AllowPartial,
				DiscardUnknown: cfg.Config.UnmarshalOptions.DiscardUnknown,
				RecursionLimit: cfg.Config.UnmarshalOptions.RecursionLimit,
			},
		}
		return s, nil
	case "proto":
		return &marshaler.ProtoMarshaller{}, nil
	default:
		return nil, errors.New("unsupported marshaler type: " + cfg.Type)
	}
}

func marshalerFromContentType(ct string) marshaler.Marshaler {
	ct = normalizeContentType(ct)
	switch ct {
	case contentTypeProto:
		return &marshaler.ProtoMarshaller{}
	case contentTypeJSON:
		return &marshaler.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}
	case "":
		return &marshaler.ProtoMarshaller{}
	default:
		if strings.Contains(ct, "json") {
			return &marshaler.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			}
		}
		return &marshaler.ProtoMarshaller{}
	}
}

func marshalerForValue(v any) marshaler.Marshaler {
	if v == nil {
		return &marshaler.ProtoMarshaller{}
	}
	if _, ok := v.(interface{ ProtoReflect() }); ok {
		return &marshaler.ProtoMarshaller{}
	}
	return &marshaler.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitUnpopulated: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
}

type marshalerCache struct {
	mu          sync.RWMutex
	inbound     marshaler.Marshaler
	outbound    marshaler.Marshaler
	inboundCfg  *MarshalerConfig
	outboundCfg *MarshalerConfig
}

func (c *marshalerCache) getInbound() marshaler.Marshaler {
	c.mu.RLock()
	if c.inbound != nil {
		m := c.inbound
		c.mu.RUnlock()
		return m
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inbound == nil {
		m, err := buildMarshaler(c.inboundCfg)
		if err == nil {
			c.inbound = m
		}
	}
	return c.inbound
}

func (c *marshalerCache) getOutbound() marshaler.Marshaler {
	c.mu.RLock()
	if c.outbound != nil {
		m := c.outbound
		c.mu.RUnlock()
		return m
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.outbound == nil {
		m, err := buildMarshaler(c.outboundCfg)
		if err == nil {
			c.outbound = m
		}
	}
	return c.outbound
}
