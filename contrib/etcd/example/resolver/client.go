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

	_ "github.com/codesjoy/yggdrasil/contrib/etcd/v2"
	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	"github.com/codesjoy/yggdrasil/v2/registry"
	"github.com/codesjoy/yggdrasil/v2/resolver"
)

type mockClient struct {
	stateCh chan resolver.State
}

func (m *mockClient) UpdateState(st resolver.State) {
	select {
	case m.stateCh <- st:
	default:
		log.Printf("[resolver] state channel full, dropping update")
	}
}

type demoInstance struct {
	namespace string
	name      string
	version   string
	region    string
	zone      string
	campus    string
	metadata  map[string]string
	endpoints []registry.Endpoint
}

func (d demoInstance) Region() string                 { return d.region }
func (d demoInstance) Zone() string                   { return d.zone }
func (d demoInstance) Campus() string                 { return d.campus }
func (d demoInstance) Namespace() string              { return d.namespace }
func (d demoInstance) Name() string                   { return d.name }
func (d demoInstance) Version() string                { return d.version }
func (d demoInstance) Metadata() map[string]string    { return d.metadata }
func (d demoInstance) Endpoints() []registry.Endpoint { return d.endpoints }

type demoEndpoint struct {
	scheme   string
	address  string
	metadata map[string]string
}

func (d demoEndpoint) Scheme() string              { return d.scheme }
func (d demoEndpoint) Address() string             { return d.address }
func (d demoEndpoint) Metadata() map[string]string { return d.metadata }

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		log.Fatalf("load config file: %v", err)
	}

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	appName := "example-resolver-client"
	if err := yggdrasil.Init(appName); err != nil {
		log.Fatalf("yggdrasil.Init: %v", err)
	}

	res, err := resolver.Get("default")
	if err != nil {
		log.Fatalf("resolver.Get: %v", err)
	}

	serviceName := "example-registry-server"
	stateCh := make(chan resolver.State, 10)
	watcher := &mockClient{stateCh: stateCh}

	if err := res.AddWatch(serviceName, watcher); err != nil {
		log.Fatalf("AddWatch: %v", err)
	}
	log.Printf("[resolver] watching service: %s", serviceName)

	reg, err := registry.Get()
	if err != nil {
		log.Fatalf("registry.Get: %v", err)
	}

	go func() {
		instanceCount := 0
		for range time.Tick(5 * time.Second) {
			instanceCount++

			inst := demoInstance{
				namespace: "default",
				name:      serviceName,
				version:   "1.0.0",
				region:    "us-west",
				zone:      "us-west-1",
				metadata: map[string]string{
					"env":         "dev",
					"instance_id": fmt.Sprintf("inst-%d", instanceCount),
				},
				endpoints: []registry.Endpoint{
					demoEndpoint{
						scheme:  "grpc",
						address: fmt.Sprintf("127.0.0.1:%d", 9000+instanceCount),
					},
					demoEndpoint{
						scheme:  "http",
						address: fmt.Sprintf("127.0.0.1:%d", 8080+instanceCount),
					},
				},
			}

			if err := reg.Register(context.Background(), inst); err != nil {
				log.Printf("[registry] register failed: %v", err)
			} else {
				log.Printf("[registry] registered instance %d (grpc://127.0.0.1:%d)",
					instanceCount, 9000+instanceCount)
			}
		}
	}()

	go func() {
		for st := range stateCh {
			eps := st.GetEndpoints()
			attrs := st.GetAttributes()

			log.Printf("[resolver] state updated")
			log.Printf("  service: %s", attrs["service"])
			log.Printf("  namespace: %s", attrs["namespace"])
			log.Printf("  revision: %v", attrs["revision"])
			log.Printf("  endpoints: %d", len(eps))

			for _, ep := range eps {
				epAttrs := ep.GetAttributes()
				log.Printf("    - %s://%s", ep.GetProtocol(), ep.GetAddress())
				log.Printf("      version: %s", epAttrs["instance_version"])
				log.Printf("      region: %s", epAttrs["instance_region"])
				log.Printf("      zone: %s", epAttrs["instance_zone"])
			}
		}
	}()

	log.Println("[client] running, press Ctrl+C to exit")

	<-stopCh
	log.Println("[client] shutting down...")

	if err := res.DelWatch(serviceName, watcher); err != nil {
		log.Printf("[resolver] del watch failed: %v", err)
	} else {
		log.Println("[resolver] stopped watching")
	}

	log.Println("[client] exited")
}
