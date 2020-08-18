// Copyright 2016 The Prometheus Authors
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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/sirupsen/logrus"
)

type Config struct {
	DataDir            string
	CORSRegex          *regexp.Regexp
	QueryTimeout       time.Duration
	QueryConcurrency   int
	QueryMaxSamples    int
	QueryLookbackDelta time.Duration
}

func NewServer(config *Config) (*http.ServeMux, error) {
	db, err := tsdb.OpenDBReadOnly(config.DataDir, nil)
	if err != nil {
		return nil, fmt.Errorf("open DB failed: %w", err)
	}

	logger := log.NewNopLogger()

	opts := promql.EngineOpts{
		Logger:                   logger,
		Reg:                      prometheus.DefaultRegisterer,
		MaxSamples:               config.QueryMaxSamples,
		Timeout:                  config.QueryTimeout,
		ActiveQueryTracker:       nil,
		LookbackDelta:            config.QueryLookbackDelta,
		NoStepSubqueryIntervalFn: nil,
	}
	queryEngine := promql.NewEngine(opts)

	api := &API{
		Queryable:   db,
		QueryEngine: queryEngine,
		logger:      logger,
		CORSOrigin:  config.CORSRegex,
	}

	mux := http.NewServeMux()

	av1 := route.New().WithInstrumentation(setPathWithPrefix("/api/v1"))
	api.Register(av1)
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", av1))

	return mux, nil
}

func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(httputil.ContextWithPath(r.Context(), prefix+r.URL.Path)))
		}
	}
}

func MustRun(mux *http.ServeMux, host string, port int) {
	logrus.Info("Starting Prometheus query server at %s:%d", host, port)
	server := &http.Server{Handler: mux, Addr: fmt.Sprintf("%s:%d", host, port)}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logrus.Fatalf("Server listen failed: %v", err)
		}
	}()

	// Wait for Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	if err := server.Shutdown(context.TODO()); err != nil {
		logrus.Fatalf("Shutdown server failed: %v", err)
	}
	wg.Wait()
}
