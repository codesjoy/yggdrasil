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
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Import the OTLP contrib module to register the exporters
	_ "github.com/codesjoy/yggdrasil/contrib/otlp/v2"
	"github.com/codesjoy/yggdrasil/v2"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	tracer = otel.Tracer("example")
	meter  = otel.Meter("example")

	// Create a counter metric
	requestCounter metric.Int64Counter
)

func init() {
	var err error
	requestCounter, err = meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		slog.Error("failed to create counter", slog.Any("error", err))
	}
}

func main() {
	// Initialize Yggdrasil (must be called once)
	yggdrasil.Init("otlp-example")

	// Start a simple HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handleRequest),
	}

	// Start server in a goroutine
	go func() {
		slog.Info("HTTP server starting", slog.String("address", ":8080"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.Any("error", err))
		}
	}()

	slog.Info("OTLP example server started on :8080")
	slog.Info("Traces and metrics are being exported to the configured OTLP endpoint")
	slog.Info("Visit http://localhost:8080 to generate traces and metrics")
	slog.Info("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", slog.Any("error", err))
	}

	slog.Info("Shutting down...")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create a span for the request
	ctx, span := tracer.Start(r.Context(), "handleRequest")
	defer span.End()

	// Add attributes to the span
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.String("http.host", r.Host),
	)

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	// Record a metric
	requestCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.Int("status", 200),
		),
	)

	// Return response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello from OTLP example!\n\n")
	fmt.Fprintf(w, "This request has been traced and metrics have been recorded.\n")
	fmt.Fprintf(w, "Check your OTLP backend to see the traces and metrics.\n")
}
