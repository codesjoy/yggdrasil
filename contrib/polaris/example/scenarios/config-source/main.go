package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/codesjoy/yggdrasil/contrib/polaris/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
)

func main() {
	if err := config.LoadSource(file.NewSource("./config.yaml", false)); err != nil {
		slog.Error("failed to load config file", slog.Any("error", err))
		os.Exit(1)
	}

	var cfg polaris.ConfigSourceConfig
	if err := config.Get(config.Join(config.KeyBase, "example", "config_source")).Scan(&cfg); err != nil {
		slog.Error("failed to scan config source config", slog.Any("error", err))
		os.Exit(1)
	}

	src, err := polaris.NewConfigSource(cfg)
	if err != nil {
		slog.Error("new polaris config source failed", slog.Any("error", err))
		os.Exit(1)
	}
	if err := config.LoadSource(src); err != nil {
		slog.Error("load polaris config source failed", slog.Any("error", err))
		os.Exit(1)
	}

	key := config.Get(config.Join(config.KeyBase, "example", "watched_key")).String()
	_ = config.AddWatcher(key, func(ev config.WatchEvent) {
		fmt.Println("type:", ev.Type(), "version:", ev.Version(), "value:", ev.Value().Int())
	})

	select {}
}
