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

// Package source provides interfaces for loading configuration
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

// Priority is the priority of the source
type Priority uint8

const (
	_ Priority = iota
	// PriorityMemory is the priority of the memory source
	PriorityMemory
	// PriorityFile is the priority of the file source
	PriorityFile
	// PriorityRemote is the priority of the remote source
	PriorityRemote
	// PriorityEnv is the priority of the env source
	PriorityEnv
	// PriorityFlag is the priority of the flag source
	PriorityFlag

	// PriorityMax is the maximum priority
	PriorityMax
)

// Data is the data returned by the source
type Data interface {
	Priority() Priority
	Data() []byte
	Unmarshal(v any) error
}

// Source is the source from which conf is loaded
type Source interface {
	Name() string
	Read() (Data, error)
	Changeable() bool
	Watch() (<-chan Data, error)
	io.Closer
}

type bytesSourceData struct {
	priority Priority
	data     []byte
	parser   Parser
}

// NewBytesSourceData creates a new bytes source data
func NewBytesSourceData(priority Priority, data []byte, parser Parser) Data {
	return &bytesSourceData{priority: priority, data: data, parser: parser}
}

func (c *bytesSourceData) Priority() Priority {
	return c.priority
}

func (c *bytesSourceData) Data() []byte {
	return c.data
}

func (c *bytesSourceData) Unmarshal(v interface{}) error {
	return c.parser(c.data, v)
}

type mapSourceData struct {
	priority Priority
	data     map[string]interface{}
}

// NewMapSourceData creates a new map source data
func NewMapSourceData(priority Priority, data map[string]interface{}) Data {
	return &mapSourceData{priority: priority, data: data}
}

func (c *mapSourceData) Priority() Priority {
	return c.priority
}

func (c *mapSourceData) Data() []byte {
	data, _ := json.Marshal(c.data)
	return data
}

func (c *mapSourceData) Unmarshal(v any) error {
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

// Parser is the parser of the source
type Parser func([]byte, any) error

// UnmarshalText implements encoding.TextUnmarshaler
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

// ParseParser parses a string to parser
func ParseParser(text string) (Parser, error) {
	var p Parser = func([]byte, any) error { return nil }
	err := p.UnmarshalText([]byte(text))
	return p, err
}
