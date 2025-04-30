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
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func main() {
	config := &config{}
	flag.StringVar(&config.lanxiHost, "lanxiHost", "169.254.61.199", "IP of the LAN-XI module")
	flag.IntVar(&config.httpPort, "httpPort", 8080, "Port of the HTTP server")
	flag.StringVar(&config.deviceID, "deviceID", "lanxi-01", "Device identifier")
	flag.StringVar(&config.location, "location", "lab-1", "Device location")
	flag.StringVar(&config.lanxiConfig, "lanxiConfig", "/home/ubuntu/lanxi/setup.json", "LAN-XI configuration file")
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
	ctx, cancel := context.WithTimeout(context.Background(),
		60*time.Second+ // reboot
			10*time.Second+ // OpenRecorder
			5*time.Second+ // CreateRecording
			10*time.Second+ // ConfigureRecording
			5*time.Second, // StartMeasurement
	)
	defer cancel()
	client := NewLANXIClient(config.lanxiHost, ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go checkLanxiAlive(config)
	fatalErrors := make(chan error, 1)

	go func() {
		logger.Info("Getting Module State")
		moduleState, err := client.GetModuleState(ctx)
		if err != nil {
			fatalErrors <- fmt.Errorf("GetModuleState failed: %w", err)
			logger.Error("Failed to get module state", "error", err)
			cancel()
			return
		}

		if moduleState != "Idle" {
			if err := client.Reboot(ctx); err != nil {
				logger.Error("Failed to reboot module", "error", err)
			} else {
				logger.Info("Rebooting module, waiting for it to restart...")
				time.Sleep(30 * time.Second)
			}
		}

		logger.Info("Opening recorder")
		if err := client.OpenRecorder(ctx); err != nil {
			logger.Error("Failed to open recorder", "error", err)
			cancel()
			return
		}
		logger.Info("Readiness Check")
		// TODO(Wesley): Implement readiness check
		if err := client.WaitForTransducerDetection(ctx); err != nil {
			logger.Error("Failed to detect transducers", "error", err)
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
		if err := client.ConfigureRecording(ctx, config); err != nil {
			fatalErrors <- fmt.Errorf("ConfigureRecording failed (critical): %w", err)
			logger.Error("ConfigureRecording failed", "error", err)
			cancel()
			return
		}
		logger.Info("Starting data stream")
		if err := client.StartStreaming(ctx); err != nil {
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
		logger.Info("Reading data stream")
		if err := client.ProcessDataStream(ctx, config); err != nil {
			logger.Error("Reading data stewam failed", "error", err)
			cancel()
			return
		}
	}()

	select {
	case <-quit:
		logger.Info("Shutting down via signal")
	case err := <-fatalErrors:
		logger.Error("Fatal error encountered - exiting", "error", err)
		// Trigger cleanup but exit with error code
		cancel()

		// Allow brief time for cleanup (optional)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}

		os.Exit(1) // Exit with non-zero status
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	logger.Info("Stopping measurement")
	if err := client.StopMeasurement(shutdownCtx); err != nil {
		logger.Error("Failed to stop measurement", "error", err)
	}
	logger.Info("Finishing Recording")
	if err := client.FinishRecording(shutdownCtx); err != nil {
		logger.Error("Failed to finish recording", "error", err)
	}
	logger.Info("Closing recorder")
	if err := client.CloseRecorder(shutdownCtx); err != nil {
		logger.Error("Failed to close recorder", "error", err)
	}
	logger.Info("Shutting down server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
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
