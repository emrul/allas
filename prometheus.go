package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type PrometheusConfig struct {
	Enabled bool
	Listen ListenConfig

	registry *prometheus.Registry
	startupTimeDesc *prometheus.Desc
	startupTimeMetric prometheus.Metric
}

func (c *PrometheusConfig) RegisterMetricsCollector(coll prometheus.Collector) error {
	if c.registry == nil {
		panic("registry not initialized")
	}
	return c.registry.Register(coll)
}

type elogWrapper struct {
}

func (w elogWrapper) Println(v ...interface{}) {
	elog.Warningf("Prometheus handler error: %s", fmt.Sprintln(v...))
}

var MetricClientConnections prometheus.Gauge
var MetricNotificationsReceived prometheus.Counter
var MetricNotificationsDispatched prometheus.Counter
var MetricListensExecuted prometheus.Counter
var MetricUnlistensExecuted prometheus.Counter
var MetricSlowClientsTerminated prometheus.Counter

func (cfg *PrometheusConfig) InitializeMetrics(r *prometheus.Registry) error {
	var err error

	cfg.startupTimeDesc = prometheus.NewDesc(
		"allas_start_time",
		"when this instance of allas was started",
		nil,
		nil,
	)
	cfg.startupTimeMetric = prometheus.MustNewConstMetric(cfg.startupTimeDesc, prometheus.GaugeValue, float64(time.Now().Unix()))
	err = r.Register(cfg)
	if err != nil {
		return err
	}

	MetricClientConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "allas",
		Name: "client_connections",
		Help: "the number of clients currently connected to allas",
	})
	err = r.Register(MetricClientConnections)
	if err != nil {
		return err
	}

	MetricNotificationsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "allas",
		Name: "notifications_received_total",
		Help: "how many notifications have been received from the PostgreSQL server so far",
	})
	err = r.Register(MetricNotificationsReceived)
	if err != nil {
		return err
	}

	MetricNotificationsDispatched = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "allas",
		Name: "notifications_dispatched_total",
		Help: "how many notifications have been dispatched so far",
	})
	err = r.Register(MetricNotificationsDispatched)
	if err != nil {
		return err
	}

	MetricListensExecuted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "allas",
		Name: "listens_executed_total",
		Help: "how many LISTEN queries have been executed so far",
	})
	err = r.Register(MetricListensExecuted)
	if err != nil {
		return err
	}

	MetricUnlistensExecuted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "allas",
		Name: "unlistens_executed_total",
		Help: "how many UNLISTEN queries have been executed so far",
	})
	err = r.Register(MetricUnlistensExecuted)
	if err != nil {
		return err
	}

	MetricSlowClientsTerminated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "allas",
		Name: "slow_clients_terminated_total",
		Help: "how many clients have been terminated because they could not keep up",
	})
	err = r.Register(MetricSlowClientsTerminated)
	if err != nil {
		return err
	}

	return nil
}

func (cfg *PrometheusConfig) Describe(ch chan<- *prometheus.Desc) {
	ch <- cfg.startupTimeDesc
}

func (cfg *PrometheusConfig) Collect(ch chan<- prometheus.Metric) {
	ch <- cfg.startupTimeMetric
}

func (cfg *PrometheusConfig) Setup() error {
	elogWrapper := elogWrapper{}
	registry := prometheus.NewPedanticRegistry()
	err := cfg.InitializeMetrics(registry)
	if err != nil {
		return err
	}
	cfg.registry = registry

	metricsHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog: elogWrapper,
	})
	muxer := http.NewServeMux()
	muxer.Handle("/metrics", metricsHandler)
	s := &http.Server{
		Handler: muxer,
	}
	l, err := cfg.Listen.Listen()
	if err != nil {
		return err
	}
	go func() {
		elog.Fatalf("Prometheus HTTP endpoint failed: %s", s.Serve(l))
	}()
	return nil
}
