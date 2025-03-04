package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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
	host string
}

func NewLANXIClient(host string) *LANXIClient {
	return &LANXIClient{host: host}
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

func (c *LANXIClient) StartStreaming(ctx context.Context) (int, error) {
	url := fmt.Sprintf("http://%s/rest/rec/destination/socket", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return int(result["tcpPort"].(float64)), nil
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
