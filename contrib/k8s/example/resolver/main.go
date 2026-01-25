package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/codesjoy/yggdrasil/contrib/k8s/v2"
	"github.com/codesjoy/yggdrasil/v2"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := yggdrasil.Init("k8s-resolver-example"); err != nil {
		panic(err)
	}

	cli, err := yggdrasil.NewClient("downstream-service")
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	slog.Info("client created, press Ctrl+C to exit...")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("exiting...")
	yggdrasil.Stop()
}
