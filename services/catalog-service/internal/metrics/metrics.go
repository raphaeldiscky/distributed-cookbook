// Package metrics defines the Prometheus metrics catalog-service emits.
//
// Service metric namespace convention (see docs/CONVENTIONS.md § 2):
//
//	namespace = cookbook
//	subsystem = <service-name underscored> (here: catalog_service)
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics bundles the request counter and latency histogram.
type Metrics struct {
	Requests *prometheus.CounterVec   // labels: route, status
	Latency  *prometheus.HistogramVec // labels: route
}

// New registers all metrics on reg and returns the bundle.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cookbook",
			Subsystem: "catalog_service",
			Name:      "requests_total",
			Help:      "Catalog-service HTTP requests labeled by route and status.",
		}, []string{"route", "status"}),
		Latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cookbook",
			Subsystem: "catalog_service",
			Name:      "request_duration_seconds",
			Help:      "Catalog-service handler latency, per route.",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 16), // ~0.1ms..3.3s
		}, []string{"route"}),
	}
	reg.MustRegister(m.Requests, m.Latency)

	return m
}
