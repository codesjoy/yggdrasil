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
	_ "github.com/codesjoy/yggdrasil/contrib/etcd/v2"
	"github.com/codesjoy/yggdrasil/v2/config"
	"github.com/codesjoy/yggdrasil/v2/config/source/file"
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

	configKey := "/example/config/blob"
	initialConfig := `server:
  port: 8080
  host: "0.0.0.0"
database:
  host: "localhost"
  port: 5432
  name: "mydb"
logging:
  level: "info"
  format: "json"
`

	_, err = cli.Put(ctx, configKey, initialConfig)
	if err != nil {
		log.Fatalf("etcd put config: %v", err)
	}
	log.Printf("[etcd] initial config written to %s", configKey)

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

	_ = config.AddWatcher("server", func(ev config.WatchEvent) {
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			var serverConfig struct {
				Port int    `mapstructure:"port"`
				Host string `mapstructure:"host"`
			}
			if err := ev.Value().Scan(&serverConfig); err != nil {
				log.Printf("[config] failed to scan server config: %v", err)
				return
			}
			log.Printf("[config] server config updated: host=%s, port=%d", serverConfig.Host, serverConfig.Port)
		}
	})

	_ = config.AddWatcher("database", func(ev config.WatchEvent) {
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			var dbConfig struct {
				Host string `mapstructure:"host"`
				Port int    `mapstructure:"port"`
				Name string `mapstructure:"name"`
			}
			if err := ev.Value().Scan(&dbConfig); err != nil {
				log.Printf("[config] failed to scan database config: %v", err)
				return
			}
			log.Printf("[config] database config updated: host=%s, port=%d, name=%s", dbConfig.Host, dbConfig.Port, dbConfig.Name)
		}
	})

	log.Println("[app] config source initialized, watching for changes...")
	log.Println("[app] press Ctrl+C to update config dynamically")
	log.Println("[app] press Ctrl+C again to exit")

	go func() {
		updateCount := 0
		for range time.Tick(10 * time.Second) {
			updateCount++
			newPort := 8080 + updateCount
			newConfig := fmt.Sprintf(`server:
  port: %d
  host: "0.0.0.0"
database:
  host: "localhost"
  port: 5432
  name: "mydb"
logging:
  level: "info"
  format: "json"
updated_at: "%s"
`, newPort, time.Now().Format(time.RFC3339))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := cli.Put(ctx, configKey, newConfig)
			cancel()
			if err != nil {
				log.Printf("[etcd] failed to update config: %v", err)
			} else {
				log.Printf("[etcd] config updated (count=%d)", updateCount)
			}
		}
	}()

	<-stopCh
	log.Println("[app] shutting down...")
}

func boolPtr(b bool) *bool {
	return &b
}
