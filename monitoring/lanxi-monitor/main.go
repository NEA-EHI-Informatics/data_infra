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
	lanxiHost string
	httpPort  int
	deviceID  string
	location  string
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func main() {
	config := &config{}
	flag.StringVar(&config.lanxiHost, "lanxiHost", "169.254.61.199", "IP of the LAN-XI module")
	flag.IntVar(&config.httpPort, "httpPort", 8080, "Port of the HTTP server")
	flag.StringVar(&config.deviceID, "deviceID", "lanxi-01", "Device identifier")
	flag.StringVar(&config.location, "location", "lab-1", "Device location")
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
		10*time.Second +    // OpenRecorder
		5*time.Second +     // CreateRecording
		10*time.Second +    // ConfigureRecording
		5*time.Second,      // StartMeasurement	
	)
	defer cancel()

	// 4. Signal handling AFTER server is running
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
		if err := client.ConfigureRecording(ctx, setup); err != nil {
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
func processDataStream(host string, port int, cfg *config) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		logger.Error("Failed to connect to data stream", "error", err)
		return
	}
	defer conn.Close()

	var (
		scaleFactors  = make(map[int32]float64)
		sampleBuffers = make(map[int32][]float64)
	)

	for {
		// Read header (28 bytes)
		header := make([]byte, 28)
		if _, err := io.ReadFull(conn, header); err != nil {
			logger.Error("Failed to read header", "error", err)
			return
		}

		// Parse header fields (little-endian)
		messageType := binary.LittleEndian.Uint32(header[8:12])
		contentLength := binary.LittleEndian.Uint32(header[12:16])

		// Read content
		content := make([]byte, contentLength)
		if _, err := io.ReadFull(conn, content); err != nil {
			logger.Error("Failed to read content", "error", err)
			return
		}

		switch messageType {
		case 1: // Interpretation message
			// Parse interpretation data (simplified example)
			signalID := int32(binary.LittleEndian.Uint32(content[0:4]))
			descType := binary.LittleEndian.Uint32(content[4:8]))
			value := math.Float64frombits(binary.LittleEndian.Uint64(content[8:16]))
			
			if descType == 3 { // Scale factor descriptor type
				scaleFactors[signalID] = value
			}

		case 2: // Signal data
			signalID := int32(binary.LittleEndian.Uint32(content[0:4]))
			numSamples := int(binary.LittleEndian.Uint32(content[4:8]))
			
			scaleFactor, ok := scaleFactors[signalID]
			if !ok {
				continue
			}

			// Parse samples (24-bit little-endian)
			samples := make([]float64, numSamples)
			for i := 0; i < numSamples; i++ {
				offset := 8 + i*3
				sample := int32(content[offset]) | int32(content[offset+1])<<8 | int32(content[offset+2])<<16
				// Sign extend if needed
				if (sample & 0x00800000) > 0 {
					sample |= ^0x00ffffff
				}
				samples[i] = float64(sample) * scaleFactor / (1 << 23)
			}

			// Update buffer
			sampleBuffers[signalID] = append(sampleBuffers[signalID], samples...)

			// Process if we have >=51000 samples
			if len(sampleBuffers[signalID]) >= 51000 {
				window := sampleBuffers[signalID][:51000]
				min, max := computeMinMax(window)
				
				// Update metrics
				lanxiAmplitudeMin.WithLabelValues(
					cfg.deviceID,
					cfg.location,
					strconv.Itoa(int(signalID)),
				).Set(min)
				
				lanxiAmplitudeMax.WithLabelValues(
					cfg.deviceID,
					cfg.location,
					strconv.Itoa(int(signalID)),
				).Set(max)

				// Keep remaining samples
				sampleBuffers[signalID] = sampleBuffers[signalID][51000:]
			}
		}
	}
}
