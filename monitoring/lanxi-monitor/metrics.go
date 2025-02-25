package main

import "github.com/prometheus/client_golang/prometheus"

var (
	lanxiUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "lanxi",
			Name:      "alive",
			Help:      "Alive status of the LAN-XI module (1 = Up, 0 = Down).",
		},
		[]string{"device_id", "location"},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(lanxiUp)
}
