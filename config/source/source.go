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

// Package source provides interfaces for loading configuration snapshots.
package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Data is a source snapshot payload.
type Data interface {
	Bytes() []byte
	Unmarshal(v any) error
}

// Source reads configuration data.
type Source interface {
	Kind() string
	Name() string
	Read() (Data, error)
	io.Closer
}

// Watchable is implemented by sources that can stream replacement snapshots.
type Watchable interface {
	Watch() (<-chan Data, error)
}

type bytesData struct {
	data   []byte
	parser Parser
}

// NewBytesData creates a new raw bytes payload.
func NewBytesData(data []byte, parser Parser) Data {
	return &bytesData{data: data, parser: parser}
}

func (c *bytesData) Bytes() []byte {
	return append([]byte(nil), c.data...)
}

func (c *bytesData) Unmarshal(v any) error {
	return c.parser(c.data, v)
}

type mapData struct {
	data map[string]any
}

// NewMapData creates a new in-memory map payload.
func NewMapData(data map[string]any) Data {
	return &mapData{data: data}
}

func (c *mapData) Bytes() []byte {
	data, _ := json.Marshal(c.data)
	return data
}

func (c *mapData) Unmarshal(v any) error {
	config := mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.TextUnmarshallerHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result: v,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return err
	}
	return decoder.Decode(c.data)
}

// Parser is the parser of the source.
type Parser func([]byte, any) error

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *Parser) UnmarshalText(text []byte) error {
	if p == nil {
		return errors.New("can't unmarshal a nil *Parser")
	}
	switch string(text) {
	case "json":
		*p = json.Unmarshal
	case "yaml", "yml":
		*p = yaml.Unmarshal
	case "toml":
		*p = toml.Unmarshal
	default:
		return fmt.Errorf("unknown parser format: %s", string(text))
	}
	return nil
}

// ParseParser parses a string to parser.
func ParseParser(text string) (Parser, error) {
	var p Parser = func([]byte, any) error { return nil }
	err := p.UnmarshalText([]byte(text))
	return p, err
}
