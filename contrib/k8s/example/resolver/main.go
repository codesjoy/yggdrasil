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
	_ = yggdrasil.Stop()
}
