// Package main implements a monitoring service for LAN-XI accelerometer modules
// exposing metrics LANXI module's health, amplitude min and max to Prometheus.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type config struct {
	lanxiHost   string
	lanxiConfig string
	httpPort    int
	deviceID    string
	location    string
	tcpPort     int
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func main() {
	config := &config{}
	flag.StringVar(&config.lanxiHost, "lanxiHost", "169.254.61.199", "IP of the LAN-XI module")
	flag.IntVar(&config.httpPort, "httpPort", 8080, "Port of the HTTP server")
	flag.StringVar(&config.deviceID, "deviceID", "lanxi-01", "Device identifier")
	flag.StringVar(&config.location, "location", "lab-1", "Device location")
	flag.StringVar(&config.lanxiConfig, "lanxiConfig", "setup.json", "LAN-XI configuration file")
	flag.Parse()

	RegisterMetrics()

	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", handleHealth).Methods("GET")

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf(":%d", config.httpPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		logger.Info("Starting HTTP server", "port", config.httpPort, "interface", "0.0.0.0")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start LAN-XI client
	client := NewLANXIClient(config.lanxiHost)
	ctx, cancel := context.WithTimeout(context.Background(),
		10*time.Second+ // OpenRecorder
			5*time.Second+ // CreateRecording
			10*time.Second+ // ConfigureRecording
			5*time.Second, // StartMeasurement
	)
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Opening recorder")
		if err := client.OpenRecorder(ctx); err != nil {
			logger.Error("Failed to open recorder", "error", err)
			cancel()
			return
		}
		logger.Info("Creating recording")
		if err := client.CreateRecording(ctx); err != nil {
			logger.Error("CreateRecording failed", "error", err)
			cancel()
			return
		}
		logger.Info("Configuring recording")
		if err := client.ConfigureRecording(ctx); err != nil {
			logger.Error("ConfigureRecording failed", "error", err)
			cancel()
			return
		}
		logger.Info("Starting measurement")
		if err := client.StartMeasurement(ctx); err != nil {
			logger.Error("StartMeasurement failed", "error", err)
			cancel()
			return
		}

	}()

	go checkLanxiAlive(config)
	// go processDataStream(config)

	<-quit
	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	logger.Info("Stopping measurement")
	if err := client.StopMeasurement(ctx); err != nil {
		logger.Error("Failed to stop measurement", "error", err)
	}
	logger.Info("Finishing Recording")
	if err := client.FinishRecording(ctx); err != nil {
		logger.Error("Failed to finish recording", "error", err)
	}
	logger.Info("Closing recorder")
	if err := client.CloseRecorder(ctx); err != nil {
		logger.Error("Failed to close recorder", "error", err)
	}
	logger.Info("Shutting down server")
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}
	shutdownCancel()
	logger.Info("Server exited properly")
}

func checkLanxiAlive(cfg *config) {
	for {
		cmd := exec.Command("ping", "-c", "1", "-W", "1", cfg.lanxiHost)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		err := cmd.Run()

		if err == nil {
			lanxiUp.WithLabelValues(cfg.deviceID, cfg.location).Set(1)
			logger.Info("LAN-XI module is reachable",
				"status", "up",
				"module", "LAN-XI",
				"address", cfg.lanxiHost,
			)
		} else {
			lanxiUp.WithLabelValues(cfg.deviceID, cfg.location).Set(0)
			logger.Error("LAN-XI module is unreachable",
				"status", "down",
				"module", "LAN-XI",
				"address", cfg.lanxiHost,
				"error", err.Error(),
			)
		}
		time.Sleep(5 * time.Second)
	}
}

func handleHealth(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("OK"))
}

func computeMinMax(samples []float64) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	min, max := samples[0], samples[0]
	for _, v := range samples {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// Add this new function to process the data stream
func processDataStream(cfg *config) {
}
