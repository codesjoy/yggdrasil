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
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/codesjoy/yggdrasil/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"gopkg.in/yaml.v3"

	"github.com/codesjoy/yggdrasil/contrib/xds/v2/example/control-plane/server"
	"github.com/codesjoy/yggdrasil/contrib/xds/v2/example/control-plane/snapshot"
	"github.com/codesjoy/yggdrasil/contrib/xds/v2/example/control-plane/watcher"
)

type Config struct {
	Server struct {
		Port   uint   `yaml:"port"`
		NodeID string `yaml:"nodeID"`
	} `yaml:"server"`
	XDS struct {
		ConfigFile    string `yaml:"configFile"`
		WatchInterval string `yaml:"watchInterval"`
	} `yaml:"xds"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
}

var snapshotVersion atomic.Uint64

func main() {
	slog.Info("Starting xDS Control Plane Server...")

	config, err := loadConfig("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	xdsConfigFile := config.XDS.ConfigFile
	if envFile := os.Getenv("XDS_CONFIG_FILE"); envFile != "" {
		xdsConfigFile = envFile
		slog.Info("Using custom xDS config file from environment", "file", xdsConfigFile)
	}

	slog.Info(
		"Server configuration loaded",
		"port",
		config.Server.Port,
		"configFile",
		xdsConfigFile,
	)

	if err := yggdrasil.Init("github.com.codesjoy.yggdrasil.contrib.xds.control-plane"); err != nil {
		slog.Error("Failed to initialize yggdrasil", "error", err)
		os.Exit(1)
	}

	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, nil)

	if err := loadAndUpdateSnapshot(xdsConfigFile, snapshotCache); err != nil {
		slog.Error("Failed to load initial xDS configuration", "error", err)
		os.Exit(1)
	}

	watchInterval := parseDuration(config.XDS.WatchInterval, 1*time.Second)
	fw, err := watcher.NewFileWatcher(xdsConfigFile, func(filePath string) {
		slog.Info("Configuration file changed, reloading", "path", filePath)
		if err := loadAndUpdateSnapshot(filePath, snapshotCache); err != nil {
			slog.Error("Failed to reload configuration", "error", err)
		}
	}, watchInterval)
	if err != nil {
		slog.Error("Failed to create file watcher", "error", err)
		os.Exit(1)
	}
	defer fw.Close()

	fw.Start()
	slog.Info("Watching configuration file", "path", xdsConfigFile)

	xdsServer := server.NewServer(config.Server.Port, snapshotCache)

	serverErr := make(chan error, 1)
	go func() {
		if err := xdsServer.Run(); err != nil {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("Received shutdown signal", "signal", sig)
	case err := <-serverErr:
		slog.Error("Server error", "error", err)
	}

	slog.Info("Shutting down xDS server...")
	xdsServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	<-ctx.Done()
	slog.Info("xDS Control Plane Server stopped")
}

//nolint:gosec // G304: File path is controlled by the application, not user input
func loadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

//nolint:gosec // G304: File path is controlled by the application, not user input
func loadAndUpdateSnapshot(filePath string, snapshotCache cache.SnapshotCache) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read xDS config file: %w", err)
	}

	var xdsConfig snapshot.XDSConfig
	if err := yaml.Unmarshal(data, &xdsConfig); err != nil {
		return fmt.Errorf("failed to parse xDS config: %w", err)
	}

	version := snapshotVersion.Add(1)
	versionStr := strconv.FormatUint(version, 10)

	slog.Info(
		"Building snapshot",
		"version",
		versionStr,
		"clusters",
		len(xdsConfig.Clusters),
		"endpoints",
		len(xdsConfig.Endpoints),
		"listeners",
		len(xdsConfig.Listeners),
		"routes",
		len(xdsConfig.Routes),
	)

	builder := snapshot.NewBuilder(versionStr)
	snap, err := builder.BuildSnapshot(&xdsConfig)
	if err != nil {
		return fmt.Errorf("failed to build snapshot: %w", err)
	}

	if err := snap.Consistent(); err != nil {
		return fmt.Errorf("snapshot inconsistency: %w", err)
	}

	if err := snapshotCache.SetSnapshot(context.Background(), "", snap); err != nil {
		return fmt.Errorf("failed to set snapshot: %w", err)
	}

	slog.Info("Snapshot updated successfully", "version", versionStr)
	return nil
}

func parseDuration(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn(
			"Failed to parse duration, using default",
			"input",
			s,
			"default",
			defaultDuration,
			"error",
			err,
		)
		return defaultDuration
	}
	return d
}
