package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lanxi-monitor/openapi"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/mjibson/go-dsp/fft"
	"github.com/mjibson/go-dsp/window"
)

type SignalData struct {
	NumberOfSignals int32
	Reserved        uint16
	Signals         []SignalBlock
}

type SignalBlock struct {
	SignalID       int32
	NumberOfValues int32
	Values         []int32
}

type frequencyAnalyzer struct {
	sampleRate  float64
	windowSize  int
	scaleFactor float64
	bufferMutex sync.Mutex
	buffer      []float64
	signalID    SignalID
	deviceID    string
	location    string
}

type LANXIClient struct {
	host       string
	client     *http.Client
	port       int
	sampleRate float64
}

func NewLANXIClient(host string, ctx context.Context) *LANXIClient {
	deadline, ok := ctx.Deadline()
	var timeout time.Duration
	if ok {
		timeout = time.Until(deadline) // Set the timeout based on the context deadline
	} else {
		timeout = 10 * time.Second // Fallback to a default value if no deadline
	}

	return &LANXIClient{
		host: host,
		client: &http.Client{
			Timeout: timeout, // Set client timeout based on context deadline
		},
	}
}

func LoadLanxiConfig(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// validation to ensure the JSON is well-formed
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	} else {
		prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
		logger.Info("Parsed config", "config", string(prettyJSON))
	}

	return data, nil
}

func (c *LANXIClient) OpenRecorder(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/open", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) GetModuleState(ctx context.Context) (string, error) {
	url := fmt.Sprintf("http://%s/rest/rec/module/info", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}

	moduleState, exists := info["moduleState"]
	if !exists {
		return "", fmt.Errorf("moduleState field not found in response")
	}

	moduleStateStr, ok := moduleState.(string)
	logger.Info("Module state", "state", moduleStateStr)
	if !ok {
		return "", fmt.Errorf("moduleState is not a string (got type %T)", moduleState)
	}

	return moduleStateStr, nil
}

func (c *LANXIClient) GetModuleInfo(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/rest/rec/module/info", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return info, nil
}

func (c *LANXIClient) DetectTEDS(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/channels/input/all/transducers/detect", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) GetTEDSInfo(ctx context.Context) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/rest/rec/channels/input/all/transducers", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var teds []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&teds); err != nil {
		return nil, err
	}
	return teds, nil
}

func (c *LANXIClient) CreateRecording(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/create", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) WaitForTransducerDetection(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Make the HTTP request
			url := fmt.Sprintf("http://%s/rest/rec/onchange", c.host)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return err
			}

			resp, err := c.client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue // Retry on transient errors
			}

			var result struct {
				Active bool `json:"transducerDetectionActive"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				continue // Retry on parse errors
			}
			resp.Body.Close()

			if !result.Active {
				return nil
			}
		}
	}
}

func (c *LANXIClient) ConfigureRecording(ctx context.Context, cfg *config) error {

	if deadline, ok := ctx.Deadline(); ok {
		logger.Info("ConfigureRecording deadline",
			"deadline", deadline.Format(time.RFC3339Nano),
			"time_remaining (s)", time.Until(deadline).Seconds(),
		)
	}

	url := fmt.Sprintf("http://%s/rest/rec/channels/input", c.host)
	jsonData, err := LoadLanxiConfig(cfg.lanxiConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	supportedRates := []float64{131072, 65536, 32768, 16384, 8192, 4096}

	// For 1.6 kHz bandwidth
	c.sampleRate = findClosestSampleRate(1600, supportedRates)
	startTime := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		// Check if the error is due to context deadline exceeded
		if errors.Is(err, context.DeadlineExceeded) {
			elapsedTime := time.Since(startTime)
			// Log the elapsed time in human-readable format
			return fmt.Errorf("context deadline exceeded after %.2f seconds for ConfigureRecording", elapsedTime.Seconds())
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response status: %d - %s", resp.Status, string(body))
	}

	// Log the time taken for the operation
	elapsedTime := time.Since(startTime)
	logger.Info("ConfigureRecording", "Duration", elapsedTime.Seconds())

	return nil
}

func (c *LANXIClient) StartStreaming(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/destination/socket", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	c.port = int(result["tcpPort"].(float64))
	return nil
	// return int(result["tcpPort"].(float64)), nil
}

func (c *LANXIClient) StartMeasurement(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/measurements", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) Reboot(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/reboot", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) StopMeasurement(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/measurements/stop", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) FinishRecording(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/finish", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) CloseRecorder(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/rest/rec/close", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *LANXIClient) ComputeMinMax(samples []float64) (float64, float64) {
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

func (c *LANXIClient) ProcessDataStream(ctx context.Context, cfg *config) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.host, c.port))
	if err != nil {
		logger.Error("Failed to connect to streaming port", "error", err)
		return err
	}
	defer conn.Close()

	brs := NewSafeStream(conn)

	scaleFactors := make(map[SignalID]float64)
	var scaleMutex sync.RWMutex

	analyzers := make(map[SignalID]*frequencyAnalyzer)
	var analyzerMutex sync.RWMutex

	// Metrics Peak frequency:
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				analyzerMutex.RLock()
				for signalID, analyzer := range analyzers {
					analyzer.bufferMutex.Lock()
					if len(analyzer.buffer) >= analyzer.windowSize {
						windowData := make([]float64, analyzer.windowSize)
						copy(windowData, analyzer.buffer[:analyzer.windowSize])
						analyzer.buffer = analyzer.buffer[analyzer.windowSize:]

						// data := make([]float64, analyzer.windowSize)
						// copy(data, analyzer.buffer[:analyzer.windowSize])
						// analyzer.buffer = analyzer.buffer[analyzer.windowSize/2:]

						go func(a *frequencyAnalyzer, sID SignalID, data []float64) {
							window.Apply(data, window.Hamming)
							fftData := fft.FFTReal(data)

							minAmp, maxAmp := math.MaxFloat64, -math.MaxFloat64
							for i := range fftData {
								re := real(fftData[i])
								// im := imag(fftData[i])
								if re > maxAmp {
									maxAmp = re
								}
								if re < minAmp {
									minAmp = re
								}

								lanxiAmplitudeMin.WithLabelValues(
									cfg.deviceID,
									cfg.location,
									fmt.Sprintf("%d", signalID),
								).Set(minAmp)
								lanxiAmplitudeMax.WithLabelValues(
									cfg.deviceID,
									cfg.location,
									fmt.Sprintf("%d", signalID),
								).Set(maxAmp)
							}
						}(analyzer, signalID, windowData)
					}
					analyzer.bufferMutex.Unlock()
				}
				analyzerMutex.RUnlock()

			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		msg := openapi.NewOpenapiMessage()
		if err := msg.Read(kaitai.NewStream(brs), nil, nil); err != nil {
			handleStreamError(err, cfg, c, conn)
			continue
		}

		switch msg.Header.MessageType {
		case openapi.OpenapiMessage_Header_EMessageType__ESignalData:
			signalData := msg.Message.(*openapi.OpenapiMessage_SignalData)
			for _, signal := range signalData.Signals {
				signalID := SignalID(uint16(signal.SignalId))

				scaleMutex.RLock()
				scaleFactor, ok := scaleFactors[signalID]
				if !ok {
					continue
				}
				scaleMutex.RUnlock()

				analyzerMutex.Lock()
				if _, exists := analyzers[signalID]; !exists {
					analyzers[signalID] = &frequencyAnalyzer{
						sampleRate: c.sampleRate, // Set during configuration
						windowSize: 4096,
						buffer:     make([]float64, 0, 2048),
						signalID:   signalID,
						deviceID:   cfg.deviceID,
						location:   cfg.location,
					}
				}
				analyzer := analyzers[signalID]
				analyzerMutex.Unlock()

				for _, value := range signal.Values {
					calcValue, _ := value.CalcValue()
					scaledValue := (float64(calcValue) * scaleFactor) / (1 << 23)

					analyzer.bufferMutex.Lock()
					analyzer.buffer = append(analyzer.buffer, scaledValue)
					analyzer.bufferMutex.Unlock()
				}
			}

		case openapi.OpenapiMessage_Header_EMessageType__EInterpretation:
			interpretations := msg.Message.(*openapi.OpenapiMessage_Interpretations)
			for _, interpretation := range interpretations.Interpretations {
				if interpretation.DescriptorType == openapi.OpenapiMessage_Interpretation_EDescriptorType__ScaleFactor {
					signalID := SignalID(interpretation.SignalId)
					if value, ok := interpretation.Value.(float64); ok {
						scaleMutex.Lock()
						scaleFactors[signalID] = value
						scaleMutex.Unlock()
					}
				}
			}
		}
	}
}

// Helper functions for better code organization
func handleStreamError(err error, cfg *config, c *LANXIClient, conn net.Conn) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		logger.Info("Stream connection closed, reconnecting...")
		conn.Close()
		reconnectWithBackoff(cfg, c)
	} else if strings.Contains(err.Error(), "exceeds maximum allowed") {
		logger.Error("Invalid message size, resetting connection")
		conn.Close()
		reconnectWithBackoff(cfg, c)
	} else {
		logger.Error("Failed to parse message", "error", err)
	}
}

func reconnectWithBackoff(cfg *config, c *LANXIClient) {
	const maxRetries = 5
	for retries := 0; retries < maxRetries; retries++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.lanxiHost, c.port))
		if err == nil {
			conn.Close()
			return
		}
		backoff := time.Duration(math.Pow(2, float64(retries))) * time.Second
		time.Sleep(backoff)
	}
	logger.Error("Failed to reconnect after multiple attempts")
}

func findClosestSampleRate(bandwidth float64, supported []float64) float64 {
	// Exactly matches Python's: abs(x - (bandwidth * 2))
	target := bandwidth * 2
	closest := supported[0]
	minDiff := math.Abs(float64(closest) - target)

	for _, rate := range supported[1:] {
		currentDiff := math.Abs(float64(rate) - target)
		if currentDiff < minDiff {
			closest = rate
			minDiff = currentDiff
		}
	}
	return closest
}
