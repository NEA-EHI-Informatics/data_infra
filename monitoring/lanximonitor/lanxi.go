package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lanxi-monitor/openapi"
	"math"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
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

type LANXIClient struct {
	host   string
	client *http.Client
	port   int
}

func NewLANXIClient(host string) *LANXIClient {
	return &LANXIClient{
		host: host,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

var (
	maxAmplitude float64
	mu           sync.Mutex
)

func updateMaxAmplitude(newValue float64) {
	mu.Lock()
	defer mu.Unlock()
	if newValue > maxAmplitude {
		maxAmplitude = newValue
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
	url := fmt.Sprintf("http://%s/rest/rec/channels/input", c.host)
	jsonData, err := LoadLanxiConfig(cfg.lanxiConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response status: %d - %s", resp.Status, string(body))
	}

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

// Add this new function to process the data stream
func (c *LANXIClient) ProcessDataStream(cfg *config) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cfg.lanxiHost, c.port))
	if err != nil {
		logger.Error("Failed to connect to streaming port", "error", err)
		return err
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
				return err
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
