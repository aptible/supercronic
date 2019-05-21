package prometheus_metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusMetrics struct {
	CronsCurrentlyRunningGauge   *prometheus.GaugeVec
	CronsExecCounter             *prometheus.CounterVec
	CronsSuccessCounter          *prometheus.CounterVec
	CronsFailCounter             *prometheus.CounterVec
	CronsDeadlineExceededCounter *prometheus.CounterVec
	CronsExecutionTimeHistogram  *prometheus.HistogramVec
	listenAddr                   string
	srv                          *http.Server
}

func New(promListenAddr string) *PrometheusMetrics {
	pm := PrometheusMetrics{}

	pm.listenAddr = promListenAddr

	pm.CronsCurrentlyRunningGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "supercronic_currently_running",
			Help: "count of currently running cron executions",
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsCurrentlyRunningGauge)

	pm.CronsExecCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "supercronic_executions",
			Help: "count of cron executions",
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsExecCounter)

	pm.CronsSuccessCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "supercronic_successful_executions",
			Help: "count of successul cron executions",
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsSuccessCounter)

	pm.CronsFailCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "supercronic_failed_executions",
			Help: "count of failed cron executions",
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsFailCounter)

	pm.CronsDeadlineExceededCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "supercronic_deadline_exceeded",
			Help: "count of exceeded deadline cron executions",
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsDeadlineExceededCounter)

	pm.CronsExecutionTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "supercronic_cron_execution_time_seconds",
			Help:    "execution times of the cron runs in buckets",
			Buckets: []float64{10.0, 30.0, 60.0, 120.0, 300.0, 600.0, 1800.0, 3600.0}, // arbitrary buckets - 10s, 30s, 60s, 120s, 300s, 600s, 1800s, 3600s)
		},
		[]string{"command", "position", "schedule"},
	)
	prometheus.MustRegister(pm.CronsExecutionTimeHistogram)

	return &pm
}

func (p *PrometheusMetrics) Reset() {
	p.CronsCurrentlyRunningGauge.Reset()
	p.CronsExecCounter.Reset()
	p.CronsSuccessCounter.Reset()
	p.CronsFailCounter.Reset()
	p.CronsDeadlineExceededCounter.Reset()
	p.CronsExecutionTimeHistogram.Reset()
}

func (p *PrometheusMetrics) InitHTTPServer() error {
	promSrv := &http.Server{Addr: p.listenAddr}

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

	p.srv = promSrv
	return promSrv.ListenAndServe()
}

func (p *PrometheusMetrics) ShutdownHTTPServer(c context.Context) error {
	return p.srv.Shutdown(c)
}
