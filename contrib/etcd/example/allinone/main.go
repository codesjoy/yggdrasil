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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil/contrib/etcd/v2"
	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/resolver"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		log.Fatalf("load config file: %v", err)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	var etcdCfg etcd.ClientConfig
	_ = config.Get("etcd.client").Scan(&etcdCfg)

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdCfg.Endpoints,
		DialTimeout: etcdCfg.DialTimeout,
		Username:    etcdCfg.Username,
		Password:    etcdCfg.Password,
	})
	if err != nil {
		log.Fatalf("etcd client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prefix := "/demo/allinone"
	configKey := prefix + "/config"

	_, err = cli.Put(ctx, configKey, "message: hello from etcd")
	if err != nil {
		log.Fatalf("etcd put config: %v", err)
	}

	var cfgSrcCfg etcd.ConfigSourceConfig
	_ = config.Get("etcd.configSource").Scan(&cfgSrcCfg)
	cfgSrc, err := etcd.NewConfigSource(cfgSrcCfg)
	if err != nil {
		log.Fatalf("etcd config source: %v", err)
	}
	defer cfgSrc.Close()

	if err := config.LoadSource(cfgSrc); err != nil {
		log.Fatalf("config.LoadSource: %v", err)
	}

	_ = config.AddWatcher("", func(ev config.WatchEvent) {
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			data := string(ev.Value().Bytes())
			log.Printf("[config] updated: %s", data)
		}
	})

	appName := "demo-allinone"
	if err := yggdrasil.Init(appName); err != nil {
		log.Fatalf("yggdrasil.Init: %v", err)
	}

	reg, err := registry.Get()
	if err != nil {
		log.Fatalf("registry.Get: %v", err)
	}

	inst := demoInstance{
		namespace: "default",
		name:      appName,
		version:   "1.0.0",
		metadata:  map[string]string{"env": "dev"},
		endpoints: []registry.Endpoint{demoEndpoint{scheme: "grpc", address: "127.0.0.1:9000"}},
	}

	if err := reg.Register(context.Background(), inst); err != nil {
		log.Fatalf("Register: %v", err)
	}
	log.Println("[registry] instance registered")

	res, err := resolver.Get("default")
	if err != nil {
		log.Fatalf("resolver.Get: %v", err)
	}

	stateCh := make(chan resolver.State, 1)
	_ = res.AddWatch(appName, &mockClient{
		stateCh: stateCh,
	})

	go func() {
		for st := range stateCh {
			eps := st.GetEndpoints()
			if len(eps) > 0 {
				log.Printf("[resolver] endpoints: %d", len(eps))
				for _, ep := range eps {
					log.Printf("  - %s://%s", ep.GetProtocol(), ep.GetAddress())
				}
			}
		}
	}()

	go func() {
		for range time.Tick(5 * time.Second) {
			newMsg := fmt.Sprintf("message: hello from etcd at %s", time.Now().Format(time.RFC3339))
			_, _ = cli.Put(context.Background(), configKey, newMsg)
		}
	}()

	<-stopCh
	log.Println("shutting down")
}

type mockClient struct {
	stateCh chan resolver.State
}

func (m *mockClient) UpdateState(st resolver.State) {
	select {
	case m.stateCh <- st:
	default:
	}
}

type demoInstance struct {
	namespace string
	name      string
	version   string
	metadata  map[string]string
	endpoints []registry.Endpoint
}

func (d demoInstance) Region() string                 { return "" }
func (d demoInstance) Zone() string                   { return "" }
func (d demoInstance) Campus() string                 { return "" }
func (d demoInstance) Namespace() string              { return d.namespace }
func (d demoInstance) Name() string                   { return d.name }
func (d demoInstance) Version() string                { return d.version }
func (d demoInstance) Metadata() map[string]string    { return d.metadata }
func (d demoInstance) Endpoints() []registry.Endpoint { return d.endpoints }

type demoEndpoint struct {
	scheme  string
	address string
}

func (d demoEndpoint) Scheme() string              { return d.scheme }
func (d demoEndpoint) Address() string             { return d.address }
func (d demoEndpoint) Metadata() map[string]string { return nil }
