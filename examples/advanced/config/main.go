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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	yapp "github.com/codesjoy/yggdrasil/v3/app"
	"github.com/codesjoy/yggdrasil/v3/config"
	"github.com/codesjoy/yggdrasil/v3/config/source/env"
	"github.com/codesjoy/yggdrasil/v3/config/source/file"
)

type AppConfig struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"server"`
	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Name     string `mapstructure:"name"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
	} `mapstructure:"database"`
	Cache struct {
		Enabled bool   `mapstructure:"enabled"`
		Host    string `mapstructure:"host"`
		Port    int    `mapstructure:"port"`
		TTL     int    `mapstructure:"ttl"`
	} `mapstructure:"cache"`
}

func main() {
	slog.Info("Starting configuration example...")

	if err := loadConfigSources(); err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	app, err := yapp.New("github.com.codesjoy.yggdrasil.example.advanced.config")
	if err != nil {
		os.Exit(1)
	}

	appConfig := &AppConfig{}
	if err := config.Default().Section("app").Decode(appConfig); err != nil {
		slog.Error("failed to scan config", slog.Any("error", err))
		os.Exit(1)
	}

	printConfig(appConfig)

	if err := validateConfig(appConfig); err != nil {
		slog.Error("config validation failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Configuration example started successfully")
	slog.Info("Press Ctrl+C to exit...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down...")
	_ = app.Stop(nil)
	slog.Info("Shutdown complete")
}

func loadConfigSources() error {
	manager := config.Default()
	if err := manager.LoadLayer("example:file", config.PriorityFile, file.NewSource("config.yaml", false)); err != nil {
		return err
	}
	slog.Info("loaded config from file")

	if err := manager.LoadLayer("example:env", config.PriorityEnv, env.NewSource([]string{"APP_"}, []string{"_"})); err != nil {
		return err
	}
	slog.Info("loaded config from env")

	return nil
}

func printConfig(cfg *AppConfig) {
	slog.Info("=== Current Configuration ===")
	slog.Info("Server", "host", cfg.Server.Host, "port", cfg.Server.Port)
	slog.Info(
		"Database",
		"host",
		cfg.Database.Host,
		"port",
		cfg.Database.Port,
		"name",
		cfg.Database.Name,
	)
	slog.Info(
		"Cache",
		"enabled",
		cfg.Cache.Enabled,
		"host",
		cfg.Cache.Host,
		"port",
		cfg.Cache.Port,
		"ttl",
		cfg.Cache.TTL,
	)
}

func validateConfig(cfg *AppConfig) error {
	if cfg.Server.Host == "" {
		return fmt.Errorf("server host is required")
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}
	return nil
}

func getConfigValue(key string) string {
	var value string
	_ = config.Default().Section(append([]string{"app"}, strings.Split(key, ".")...)...).Decode(&value)
	return value
}

func getServerHost() string {
	var host string
	if err := config.Default().Section("app", "server", "host").Decode(&host); err != nil || host == "" {
		return "localhost"
	}
	return host
}

func getServerPort() int {
	var port int
	if err := config.Default().Section("app", "server", "port").Decode(&port); err != nil || port == 0 {
		return 8080
	}
	return port
}

func isCacheEnabled() bool {
	var enabled bool
	_ = config.Default().Section("app", "cache", "enabled").Decode(&enabled)
	return enabled
}
