package infra

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// TypeHTTP ...
	TypeHTTP = "http"
)

var (
	MetricsReg = prometheus.NewRegistry()
	hostname   string
	err        error
	app        string
)

func init() {
	hostname, err = os.Hostname()
	if err != nil {
		log.Error("hostname get fail err=%s", err)
	}
	app = filepath.Base(os.Args[0])
	MetricsReg.MustRegister(serverHandleHistogram)
}

var (
	serverHandleHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "server_handle_seconds",
	}, []string{"type", "method", "status", "api", "app", "host"})
)

func RecordServerHandlerSeconds(metricType string, method string, status int, api string, second float64) {
	serverHandleHistogram.With(prometheus.Labels{
		"type":   metricType,
		"method": method,
		"status": strconv.Itoa(status),
		"api":    api,
		"host":   hostname,
		"app":    app,
	}).Observe(second)
}

func RegPrometheusClient() {
	http.Handle("/metrics", promhttp.HandlerFor(MetricsReg, promhttp.HandlerOpts{Registry: MetricsReg}))
	go func() {
		http.ListenAndServe(":8090", http.DefaultServeMux)
	}()
}
