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
	lanxiAmplitudeMin = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "lanxi",
			Name:      "amplitude_min",
			Help:      "Minimum amplitude per second.",
		},
		[]string{"device_id", "location", "channel"},
	)
	lanxiAmplitudeMax = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "lanxi",
			Name:      "amplitude_max",
			Help:      "Maximum amplitude per second.",
		},
		[]string{"device_id", "location", "channel"},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(
		lanxiUp,
		lanxiAmplitudeMin,
		lanxiAmplitudeMax,
	)
}
