package prometheus_metrics

import (
	"context"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	namespace = "supercronic"
)

func genMetricName(name string) string {
	return prometheus.BuildFQName(namespace, "", name)
}

type PrometheusMetrics struct {
	CronsCurrentlyRunningGauge   prometheus.GaugeVec
	CronsExecCounter             prometheus.CounterVec
	CronsSuccessCounter          prometheus.CounterVec
	CronsFailCounter             prometheus.CounterVec
	CronsDeadlineExceededCounter prometheus.CounterVec
	CronsExecutionTimeHistogram  prometheus.HistogramVec
}

func NewPrometheusMetrics() PrometheusMetrics {
	cronLabels := []string{"command", "position", "schedule"}

	pm := PrometheusMetrics{}

	pm.CronsCurrentlyRunningGauge = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: genMetricName("currently_running"),
			Help: "count of currently running cron executions",
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsCurrentlyRunningGauge)

	pm.CronsExecCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: genMetricName("executions"),
			Help: "count of cron executions",
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsExecCounter)

	pm.CronsSuccessCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: genMetricName("successful_executions"),
			Help: "count of successul cron executions",
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsSuccessCounter)

	pm.CronsFailCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: genMetricName("failed_executions"),
			Help: "count of failed cron executions",
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsFailCounter)

	pm.CronsDeadlineExceededCounter = *prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: genMetricName("deadline_exceeded"),
			Help: "count of exceeded deadline cron executions",
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsDeadlineExceededCounter)

	pm.CronsExecutionTimeHistogram = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    genMetricName("cron_execution_time_seconds"),
			Help:    "duration of the cron executions",
			Buckets: []float64{10.0, 30.0, 60.0, 120.0, 300.0, 600.0, 1800.0, 3600.0},
		},
		cronLabels,
	)
	prometheus.MustRegister(pm.CronsExecutionTimeHistogram)

	return pm
}

func (p *PrometheusMetrics) Reset() {
	p.CronsCurrentlyRunningGauge.Reset()
	p.CronsExecCounter.Reset()
	p.CronsSuccessCounter.Reset()
	p.CronsFailCounter.Reset()
	p.CronsDeadlineExceededCounter.Reset()
	p.CronsExecutionTimeHistogram.Reset()
}

func InitHTTPServer(listenAddr string, shutdownContext context.Context) (func() error, error) {
	promSrv := &http.Server{}

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Supercronic</title></head>
             <body>
             <h1>Supercronic</h1>
             <p><a href='/metrics'>Metrics</a></p>
             </body>
             </html>`))
	})

	shutdownClosure := func() error {
		return promSrv.Shutdown(shutdownContext)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return shutdownClosure, err
	}

	go func() {
		if err := promSrv.Serve(listener); err != nil {
			logrus.Fatalf("prometheus http serve failed: %s", err.Error())
		}
	}()

	return shutdownClosure, nil
}
