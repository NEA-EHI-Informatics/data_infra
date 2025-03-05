package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"lanxi-monitor/openapi"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
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
		if err := client.ConfigureRecording(ctx, config); err != nil {
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

type bufferedReadSeeker struct {
	r   io.Reader
	buf *bytes.Buffer
}

func newBufferedReadSeeker(r io.Reader) *bufferedReadSeeker {
	return &bufferedReadSeeker{
		r:   r,
		buf: bytes.NewBuffer(nil),
	}
}

func (b *bufferedReadSeeker) Read(p []byte) (n int, err error) {
	// First try to read from the buffer
	if b.buf.Len() > 0 {
		return b.buf.Read(p)
	}

	// If buffer is empty, read from the source
	return b.r.Read(p)
}

func (b *bufferedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	// Only support seeking from current position (relative seeks)
	if whence != io.SeekCurrent {
		return 0, fmt.Errorf("only SeekCurrent is supported")
	}

	// If seeking forward, discard bytes
	if offset > 0 {
		_, err := io.CopyN(io.Discard, b, offset)
		return offset, err
	}

	// If seeking backward, we need to have buffered enough data
	if b.buf.Len() < int(-offset) {
		return 0, fmt.Errorf("cannot seek back beyond buffered data")
	}

	// Move the read position back
	newBuf := bytes.NewBuffer(b.buf.Bytes()[:b.buf.Len()+int(offset)])
	b.buf = newBuf
	return offset, nil
}

type SignalID uint16

// Add this new function to process the data stream
func processDataStream(cfg *config, port int) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.lanxiHost, port))
	if err != nil {
		logger.Error("Failed to connect to streaming port", "error", err)
		return
	}
	defer conn.Close()
	brs := newBufferedReadSeeker(conn)
	scaleFactors := make(map[SignalID]float64)
	var scaleMutex sync.RWMutex

	type channelStats struct {
		min, max float64
		count    int
	}
	stats := make(map[SignalID]*channelStats)
	var statsMutex sync.Mutex

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Metrics updater
	go func() {
		for range ticker.C {
			statsMutex.Lock()
			for signalID, stat := range stats {
				if stat.count == 0 {
					continue
				}

				scaleMutex.RLock()
				scaleFactor, ok := scaleFactors[signalID]
				scaleMutex.RUnlock()

				if ok {
					lanxiAmplitudeMin.WithLabelValues(
						cfg.deviceID,
						cfg.location,
						fmt.Sprintf("%d", signalID),
					).Set(stat.min * scaleFactor)

					lanxiAmplitudeMax.WithLabelValues(
						cfg.deviceID,
						cfg.location,
						fmt.Sprintf("%d", signalID),
					).Set(stat.max * scaleFactor)
				}

				// Reset stats
				stat.min = math.MaxFloat64
				stat.max = -math.MaxFloat64
				stat.count = 0
			}
			statsMutex.Unlock()
		}
	}()

	for {
		// Read message using Kaitai parser
		msg := openapi.NewOpenapiMessage()
		err = msg.Read(kaitai.NewStream(brs), nil, nil)
		if err != nil {
			if err == io.EOF {
				logger.Info("Stream connection closed")
				return
			}
			logger.Error("Failed to parse message", "error", err)
			continue
		}

		switch msg.Header.MessageType {
		case openapi.OpenapiMessage_Header_EMessageType__ESignalData:
			signalData := msg.Message.(*openapi.OpenapiMessage_SignalData)
			for _, signal := range signalData.Signals {
				signalID := SignalID(uint16(signal.SignalId))
				scaleMutex.RLock()
				scaleFactor, ok := scaleFactors[signalID]
				scaleMutex.RUnlock()

				if !ok {
					continue
				}

				statsMutex.Lock()
				stat, exists := stats[signalID]
				if !exists {
					stat = &channelStats{
						min: math.MaxFloat64,
						max: -math.MaxFloat64,
					}
					stats[signalID] = stat
				}
				statsMutex.Unlock()

				for _, value := range signal.Values {
					calcValue, _ := value.CalcValue()
					scaledValue := float64(calcValue) * scaleFactor

					if scaledValue < stat.min {
						stat.min = scaledValue
					}
					if scaledValue > stat.max {
						stat.max = scaledValue
					}
					stat.count++
				}
			}

		case openapi.OpenapiMessage_Header_EMessageType__EInterpretation:
			interpretations := msg.Message.(*openapi.OpenapiMessage_Interpretations)
			for _, interpretation := range interpretations.Interpretations {
				if interpretation.DescriptorType == openapi.OpenapiMessage_Interpretation_EDescriptorType__ScaleFactor {
					signalID := SignalID(interpretation.SignalId)
					scaleMutex.Lock()
					scaleFactors[signalID] = interpretation.Value.(float64)
					scaleMutex.Unlock()
				}
			}

		case openapi.OpenapiMessage_Header_EMessageType__EDataQuality:
			// Handle data quality messages if needed
			dataQuality := msg.Message.(*openapi.OpenapiMessage_DataQuality)
			for _, quality := range dataQuality.Qualities {
				if overload, _ := quality.ValidityFlags.Overload(); overload {
					logger.Warn("Signal overload detected", "signal_id", quality.SignalId)
				}
			}
		}
	}
}
