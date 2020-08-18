package main

import (
	"os"
	"regexp"
	"time"

	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/breeswish/tidb-metrics-viewer/pkg/server"
)

var (
	host string
	port int
)

func rootCmdRun(cmd *cobra.Command, args []string) {
	mux, err := server.NewServer(&server.Config{
		DataDir:            args[0],
		CORSRegex:          mustCompileCORSRegexString(".*"),
		QueryTimeout:       2 * time.Minute,
		QueryConcurrency:   20,
		QueryMaxSamples:    50000000,
		QueryLookbackDelta: 5 * time.Minute,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	server.MustRun(mux, host, port)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "tidb-metrics-viewer [prometheus_dump_dir]",
		Short: "Start a Prometheus and Grafana server from a Prometheus dump",
		Args:  cobra.MinimumNArgs(1),
		Run:   rootCmdRun,
	}
	rootCmd.Flags().StringVarP(&host, "host", "h", "0.0.0.0",
		"Listen host")
	rootCmd.Flags().IntVarP(&port, "port", "p", 14332,
		"Listen port")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func mustCompileCORSRegexString(s string) *regexp.Regexp {
	r, err := relabel.NewRegexp(s)
	if err != nil {
		panic(err)
	}
	return r.Regexp
}
