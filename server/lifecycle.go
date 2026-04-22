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

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/codesjoy/yggdrasil/v3/remote"
)

func (s *server) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.beginStop() {
		return nil
	}

	var (
		errs error
		mu   sync.Mutex
		wg   sync.WaitGroup
	)
	appendErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		errs = errors.Join(errs, err)
		mu.Unlock()
	}

	for _, item := range s.servers {
		wg.Add(1)
		go func(srv remote.Server) {
			defer wg.Done()
			if err := srv.Stop(ctx); err != nil {
				appendErr(err)
				slog.Error("fault to stop server",
					slog.String("protocol", srv.Info().Protocol),
					slog.Any("error", err))
			}
		}(item)
	}

	if s.restEnable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.restSvr.Stop(ctx); err != nil {
				appendErr(err)
				slog.Error("fault to stop rest server",
					slog.Any("error", err))
			}
		}()
	}

	wg.Wait()
	return errs
}

func (s *server) Serve(startFlag chan<- struct{}) (err error) {
	runtimeErrCh := make(chan error, len(s.servers)+1)

	defer func() {
		if err != nil {
			_ = s.Stop(context.Background())
		}
		if startFlag != nil {
			close(startFlag)
		}
	}()
	if err = s.beginServe(); err != nil {
		return err
	}

	for _, svr := range s.servers {
		if err = s.serve(svr, runtimeErrCh); err != nil {
			return err
		}
	}

	if err = s.restServe(runtimeErrCh); err != nil {
		return err
	}

	if startFlag != nil {
		startFlag <- struct{}{}
	}
	return s.waitForServeResult(runtimeErrCh)
}

func (s *server) serve(svr remote.Server, runtimeErrCh chan<- error) error {
	err := svr.Start()
	if err != nil {
		slog.Error(
			"fault to start server",
			slog.String("protocol", svr.Info().Protocol),
			slog.Any("error", err),
		)
		return err
	}
	slog.Info(
		"server started",
		slog.String("protocol", svr.Info().Protocol),
		slog.String("endpoint", svr.Info().Address),
	)
	s.serverWG.Add(1)
	go func() {
		defer s.serverWG.Done()
		if handleErr := svr.Handle(); handleErr != nil && !errors.Is(handleErr, http.ErrServerClosed) {
			slog.Error(
				"the server exits abnormally",
				slog.String("protocol", svr.Info().Protocol),
				slog.Any("error", handleErr),
			)
			s.reportServeRuntimeError(
				runtimeErrCh,
				fmt.Errorf("server %s exited abnormally: %w", svr.Info().Protocol, handleErr),
			)
		}
	}()
	return nil
}

func (s *server) restServe(runtimeErrCh chan<- error) error {
	if !s.restEnable {
		return nil
	}
	err := s.restSvr.Start()
	if err != nil {
		slog.Error("fault to start rest server", slog.Any("error", err))
		return err
	}
	slog.Info("rest server started", slog.String("endpoint", s.restSvr.Info().GetAddress()))
	s.serverWG.Add(1)
	go func() {
		defer s.serverWG.Done()
		if serveErr := s.restSvr.Serve(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("fault to serve rest server", slog.Any("error", serveErr))
			s.reportServeRuntimeError(runtimeErrCh, fmt.Errorf("rest server exited abnormally: %w", serveErr))
		}
	}()
	return nil
}

func (s *server) reportServeRuntimeError(ch chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case ch <- err:
	default:
	}
}

func (s *server) beginStop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case serverStateInit:
		s.state = serverStateClosing
		return false
	case serverStateClosing:
		return false
	default:
		s.state = serverStateClosing
		return true
	}
}

func (s *server) beginServe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case serverStateClosing:
		return errors.New("server stopped")
	case serverStateRunning:
		return errors.New("server already serve")
	}
	if s.registerErr != nil {
		return fmt.Errorf("server registration failed: %w", s.registerErr)
	}
	s.state = serverStateRunning
	return nil
}

func (s *server) waitForServeResult(runtimeErrCh <-chan error) error {
	done := make(chan struct{})
	go func() {
		s.serverWG.Wait()
		close(done)
	}()

	for {
		select {
		case err := <-runtimeErrCh:
			return err
		case <-done:
			select {
			case err := <-runtimeErrCh:
				return err
			default:
				return nil
			}
		}
	}
}
