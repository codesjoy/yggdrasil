package main

import (
	"fmt"
	"os"

	k8s "github.com/codesjoy/yggdrasil/contrib/k8s/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source"
)

func main() {
	src, err := k8s.NewConfigMapSource(k8s.ConfigSourceConfig{
		Namespace: "default",
		Name:      "example-config",
		Key:       "config.yaml",
		Watch:     true,
		Priority:  source.PriorityRemote,
	})
	if err != nil {
		panic(err)
	}
	if err := config.LoadSource(src); err != nil {
		panic(err)
	}

	if err := config.AddWatcher("example", func(ev config.WatchEvent) {
		fmt.Printf("config changed: type=%v, version=%d\n", ev.Type(), ev.Version())
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			var cfg struct {
				Message string `mapstructure:"message"`
			}
			if err := ev.Value().Scan(&cfg); err != nil {
				fmt.Printf("failed to scan config: %v\n", err)
				return
			}
			fmt.Printf("message: %s\n", cfg.Message)
		}
	}); err != nil {
		panic(err)
	}

	fmt.Println("watching config changes, press Ctrl+C to exit...")
	sig := make(chan os.Signal, 1)
	<-sig
	fmt.Println("exiting...")
}
