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

package polaris

import (
	"testing"
	"time"

	polaris "github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type fakeConfigAPI struct {
	file model.ConfigFile
	err  error
}

func (f *fakeConfigAPI) FetchConfigFile(*polaris.GetConfigFileRequest) (model.ConfigFile, error) {
	return f.file, f.err
}

type fakeConfigFile struct {
	namespace string
	group     string
	name      string
	mode      model.GetConfigFileRequestMode
	content   string
	ch        chan model.ConfigFileChangeEvent
}

func (f *fakeConfigFile) GetNamespace() string { return f.namespace }
func (f *fakeConfigFile) GetFileGroup() string { return f.group }
func (f *fakeConfigFile) GetFileName() string  { return f.name }
func (f *fakeConfigFile) GetFileMode() model.GetConfigFileRequestMode {
	return f.mode
}

func (f *fakeConfigFile) GetLabels() map[string]string { return map[string]string{} }
func (f *fakeConfigFile) GetContent() string           { return f.content }
func (f *fakeConfigFile) HasContent() bool             { return f.content != "" }
func (f *fakeConfigFile) AddChangeListenerWithChannel() <-chan model.ConfigFileChangeEvent {
	return f.ch
}

func (f *fakeConfigFile) AddChangeListener(cb model.OnConfigFileChange) {
	go func() {
		for ev := range f.ch {
			cb(ev)
		}
	}()
}
func (f *fakeConfigFile) GetPersistent() model.Persistent { return model.Persistent{} }

func TestConfigSource_ReadAndWatch(t *testing.T) {
	file := &fakeConfigFile{
		namespace: "default",
		group:     "app",
		name:      "service.yaml",
		mode:      model.SDKMode,
		content:   "yggdrasil:\n  rest:\n    enable: true\n",
		ch:        make(chan model.ConfigFileChangeEvent, 1),
	}
	src, err := NewConfigSource(ConfigSourceConfig{
		FileName:  "service.yaml",
		FileGroup: "app",
		API:       &fakeConfigAPI{file: file},
	})
	if err != nil {
		t.Fatalf("NewConfigSource err: %v", err)
	}
	defer src.Close()

	data, err := src.Read()
	if err != nil {
		t.Fatalf("Read err: %v", err)
	}
	var m map[string]any
	if err := data.Unmarshal(&m); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	y, ok := m["yggdrasil"].(map[string]any)
	if !ok {
		t.Fatalf("expected yggdrasil map, got %T", m["yggdrasil"])
	}
	rest, ok := y["rest"].(map[string]any)
	if !ok || rest["enable"] != true {
		t.Fatalf("expected rest.enable true, got %#v", y["rest"])
	}

	wch, err := src.Watch()
	if err != nil {
		t.Fatalf("Watch err: %v", err)
	}
	file.ch <- model.ConfigFileChangeEvent{NewValue: "yggdrasil:\n  rest:\n    enable: false\n"}

	select {
	case d := <-wch:
		var mm map[string]any
		if err := d.Unmarshal(&mm); err != nil {
			t.Fatalf("watch Unmarshal err: %v", err)
		}
		yy := mm["yggdrasil"].(map[string]any)
		rr := yy["rest"].(map[string]any)
		if rr["enable"] != false {
			t.Fatalf("expected rest.enable false, got %#v", rr["enable"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}
