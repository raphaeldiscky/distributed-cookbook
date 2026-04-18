// Package metrics defines the Prometheus metrics the flashsale recipe emits.
// No package-level globals — all metrics are constructed against an injected
// registry, so tests and the server don't share state.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics bundles the counters/histograms/gauges emitted by the flashsale recipe.
//
// Every variant-sensitive metric carries an `adapter` label (see
// CONVENTIONS.md § 2) so Grafana panels can split the three adapters
// side-by-side on one dashboard.
type Metrics struct {
	OversellTotal    *prometheus.CounterVec   // labels: adapter
	CheckoutAttempts *prometheus.CounterVec   // labels: adapter, outcome (ok|out_of_stock|not_found|error)
	CheckoutLatency  *prometheus.HistogramVec // labels: adapter
	StockRemaining   *prometheus.GaugeVec     // labels: adapter, product_id
}

// New registers all metrics on reg and returns the bundle.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		OversellTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cookbook",
			Subsystem: "flashsale",
			Name:      "oversell_total",
			Help:      "Successful checkouts beyond the seeded stock, per adapter. MUST be 0 for correct adapters.",
		}, []string{"adapter"}),
		CheckoutAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cookbook",
			Subsystem: "flashsale",
			Name:      "checkout_attempts_total",
			Help:      "Checkout attempts labeled by adapter and outcome.",
		}, []string{"adapter", "outcome"}),
		CheckoutLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cookbook",
			Subsystem: "flashsale",
			Name:      "checkout_latency_seconds",
			Help:      "End-to-end checkout handler latency, per adapter.",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 2, 14), // ~0.5ms..8s
		}, []string{"adapter"}),
		StockRemaining: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "cookbook",
			Subsystem: "flashsale",
			Name:      "stock_remaining",
			Help:      "Stock remaining per (adapter, product) — sampled after every successful checkout.",
		}, []string{"adapter", "product_id"}),
	}
	reg.MustRegister(m.OversellTotal, m.CheckoutAttempts, m.CheckoutLatency, m.StockRemaining)

	return m
}
