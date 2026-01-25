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

	configPrefix := "/example/config/kv"

	ops := []clientv3.Op{
		clientv3.OpPut(configPrefix+"/server/port", "8080"),
		clientv3.OpPut(configPrefix+"/server/host", "0.0.0.0"),
		clientv3.OpPut(configPrefix+"/server/readTimeout", "30s"),
		clientv3.OpPut(configPrefix+"/database/host", "localhost"),
		clientv3.OpPut(configPrefix+"/database/port", "5432"),
		clientv3.OpPut(configPrefix+"/database/name", "mydb"),
		clientv3.OpPut(configPrefix+"/database/poolSize", "10"),
		clientv3.OpPut(configPrefix+"/logging/level", "info"),
		clientv3.OpPut(configPrefix+"/logging/format", "json"),
		clientv3.OpPut(configPrefix+"/cache/redis/host", "localhost"),
		clientv3.OpPut(configPrefix+"/cache/redis/port", "6379"),
		clientv3.OpPut(configPrefix+"/cache/ttl", "300"),
	}
	_, err = cli.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		log.Fatalf("etcd put config: %v", err)
	}
	log.Printf("[etcd] initial config written to %s/*", configPrefix)

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
				Port        int    `mapstructure:"port"`
				Host        string `mapstructure:"host"`
				ReadTimeout string `mapstructure:"readTimeout"`
			}
			if err := ev.Value().Scan(&serverConfig); err != nil {
				log.Printf("[config] failed to scan server config: %v", err)
				return
			}
			log.Printf("[config] server config updated: host=%s, port=%d, timeout=%s",
				serverConfig.Host, serverConfig.Port, serverConfig.ReadTimeout)
		}
	})

	_ = config.AddWatcher("database", func(ev config.WatchEvent) {
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			var dbConfig struct {
				Host     string `mapstructure:"host"`
				Port     int    `mapstructure:"port"`
				Name     string `mapstructure:"name"`
				PoolSize int    `mapstructure:"poolSize"`
			}
			if err := ev.Value().Scan(&dbConfig); err != nil {
				log.Printf("[config] failed to scan database config: %v", err)
				return
			}
			log.Printf("[config] database config updated: host=%s, port=%d, name=%s, pool=%d",
				dbConfig.Host, dbConfig.Port, dbConfig.Name, dbConfig.PoolSize)
		}
	})

	_ = config.AddWatcher("cache", func(ev config.WatchEvent) {
		if ev.Type() == config.WatchEventUpd || ev.Type() == config.WatchEventAdd {
			var cacheConfig struct {
				TTL int `mapstructure:"ttl"`
			}
			if err := ev.Value().Scan(&cacheConfig); err != nil {
				log.Printf("[config] failed to scan cache config: %v", err)
				return
			}
			log.Printf("[config] cache config updated: ttl=%d seconds", cacheConfig.TTL)
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
			newPoolSize := 10 + updateCount
			newTTL := 300 + updateCount*60

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := cli.Put(ctx, configPrefix+"/server/port", fmt.Sprintf("%d", newPort))
			cancel()
			if err != nil {
				log.Printf("[etcd] failed to update server/port: %v", err)
			}

			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			_, err = cli.Put(ctx, configPrefix+"/database/poolSize", fmt.Sprintf("%d", newPoolSize))
			cancel()
			if err != nil {
				log.Printf("[etcd] failed to update database/poolSize: %v", err)
			}

			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			_, err = cli.Put(ctx, configPrefix+"/cache/ttl", fmt.Sprintf("%d", newTTL))
			cancel()
			if err != nil {
				log.Printf("[etcd] failed to update cache/ttl: %v", err)
			} else {
				log.Printf("[etcd] partial config updated (count=%d)", updateCount)
			}
		}
	}()

	<-stopCh
	log.Println("[app] shutting down...")
}

func boolPtr(b bool) *bool {
	return &b
}
