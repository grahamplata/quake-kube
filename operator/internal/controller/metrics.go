package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// QuakeServerActiveTotal is a gauge for total number of QuakeServer resources.
	QuakeServerActiveTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "quakeserver_active_total",
			Help: "Total number of QuakeServer resources defined in the cluster.",
		},
	)

	// QuakeServerPlayersActive is a gauge for active players, labeled by server_name and namespace.
	QuakeServerPlayersActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "quakeserver_players_active",
			Help: "Number of active players on a specific QuakeServer.",
		},
		[]string{"server_name", "namespace"},
	)

	// QuakeServerReconciliationTotal is a counter for reconciliation results.
	QuakeServerReconciliationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "quakeserver_reconciliation_total",
			Help: "Total number of QuakeServer reconciliations.",
		},
		[]string{"result"}, // success, failure
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		QuakeServerActiveTotal,
		QuakeServerPlayersActive,
		QuakeServerReconciliationTotal,
	)
}
