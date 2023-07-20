package infra

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"regexp"
	"strconv"
)

const (
	// TypeHTTP ...
	TypeHTTP = "http"

	TypeMySQL = "mysql"

	TypeRedis = "redis"
)

var (
	MetricsReg = prometheus.NewRegistry()
)

func init() {
	MetricsReg.MustRegister(serverHandleHistogram, serverHandleCounter, clientHandleHistogram, clientHandleCounter)
	MetricMonitor.RegPrometheusClient()
}

type metricMonitor struct {
}

var MetricMonitor = &metricMonitor{}

var (
	serverHandleHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "server_handle_seconds",
	}, []string{"type", "method", "status", "api"})

	serverHandleCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "server_handle_total",
	}, []string{"type", "method", "api"})

	clientHandleCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "client_handle_total",
	}, []string{"type", "name", "op", "peer"})

	clientHandleHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "client_handle_seconds",
	}, []string{"type", "name", "op", "peer"})
)

func RecordClientCount(metricType string, method string, name string, peer string) {
	clientHandleCounter.With(prometheus.Labels{
		"type": metricType,
		"op":   method,
		"name": name,
		"peer": peer,
	}).Inc()
}

func (m *metricMonitor) RecordClientHandlerSeconds(metricType string, method, name string, peer string, second float64) {
	clientHandleHistogram.With(prometheus.Labels{
		"type": metricType,
		"op":   method,
		"peer": peer,
		"name": name,
	}).Observe(second)
}

func (m *metricMonitor) RecordServerHandlerSeconds(metricType string, method string, status int, api string, second float64) {
	serverHandleHistogram.With(prometheus.Labels{
		"type":   metricType,
		"method": method,
		"status": strconv.Itoa(status),
		"api":    api,
	}).Observe(second)
}

func (m *metricMonitor) RecordServerCount(metricType string, method string, api string) {
	serverHandleCounter.With(prometheus.Labels{
		"type":   metricType,
		"method": method,
		"api":    api,
	}).Inc()
}

func (m *metricMonitor) RegPrometheusClient() {
	MetricsReg.MustRegister(
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	http.Handle("/metrics", promhttp.HandlerFor(MetricsReg, promhttp.HandlerOpts{Registry: MetricsReg}))
	go func() {
		http.ListenAndServe(":8090", http.DefaultServeMux)
	}()
}
