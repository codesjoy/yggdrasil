package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/codesjoy/yggdrasil/contrib/etcd/v2"
	"github.com/codesjoy/yggdrasil/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
	"github.com/codesjoy/yggdrasil/v2/registry"
)

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

	appName := "example-registry-server"
	if err := yggdrasil.Init(appName); err != nil {
		log.Fatalf("yggdrasil.Init: %v", err)
	}

	reg, err := registry.Get()
	if err != nil {
		log.Fatalf("registry.Get: %v", err)
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	log.Printf("[server] listening on %s", addr)

	inst := demoInstance{
		namespace: "default",
		name:      appName,
		version:   "1.0.0",
		region:    "us-west",
		zone:      "us-west-1",
		campus:    "campus-a",
		metadata: map[string]string{
			"env":     "dev",
			"pod":     fmt.Sprintf("pod-%d", time.Now().Unix()),
			"started": time.Now().Format(time.RFC3339),
		},
		endpoints: []registry.Endpoint{
			demoEndpoint{scheme: "grpc", address: addr},
			demoEndpoint{scheme: "http", address: addr},
		},
	}

	if err := reg.Register(context.Background(), inst); err != nil {
		log.Fatalf("Register: %v", err)
	}
	log.Println("[registry] instance registered successfully")
	log.Printf("[registry] service: %s/%s/%s", inst.Namespace(), inst.Name(), inst.Version())
	log.Printf("[registry] endpoints: %d", len(inst.Endpoints()))
	for _, ep := range inst.Endpoints() {
		log.Printf("  - %s://%s", ep.Scheme(), ep.Address())
	}

	log.Println("[server] running, press Ctrl+C to shutdown")

	go func() {
		for range time.Tick(30 * time.Second) {
			inst.metadata["heartbeat"] = time.Now().Format(time.RFC3339)
			if err := reg.Register(context.Background(), inst); err != nil {
				log.Printf("[registry] re-register failed: %v", err)
			} else {
				log.Println("[registry] instance re-registered")
			}
		}
	}()

	<-stopCh
	log.Println("[server] shutting down...")

	if err := reg.Deregister(context.Background(), inst); err != nil {
		log.Printf("[registry] deregister failed: %v", err)
	} else {
		log.Println("[registry] instance deregistered successfully")
	}

	_ = ln.Close()
	log.Println("[server] exited")
}
