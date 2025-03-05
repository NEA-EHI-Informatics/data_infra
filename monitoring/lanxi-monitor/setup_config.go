package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Struct to match the JSON structure
type LanxiConfig struct {
	Channels      []Channel `json:"channels"`
	FileFormat    string    `json:"fileFormat"`
	MaxSize       int       `json:"maxSize"`
	Name          string    `json:"name"`
	RecordingMode string    `json:"recordingMode"`
}

type Channel struct {
	ConnectorSelect                  string     `json:"ConnectorSelect"`
	Bandwidth                        string     `json:"bandwidth"`
	BridgeCompletion                 string     `json:"bridgeCompletion"`
	BridgeExcCurrent                 float64    `json:"bridgeExcCurrent"`
	BridgeExcOn                      bool       `json:"bridgeExcOn"`
	BridgeExcVoltage                 float64    `json:"bridgeExcVoltage"`
	BridgeQuarterCompletionImpedance string     `json:"bridgeQuarterCompletionImpedance"`
	BridgeRemoteSenseWiring          string     `json:"bridgeRemoteSenseWiring"`
	BridgeShunt                      bool       `json:"bridgeShunt"`
	BridgeSingleEnd                  bool       `json:"bridgeSingleEnd"`
	BridgeSupplyType                 string     `json:"bridgeSupplyType"`
	Ccld                             bool       `json:"ccld"`
	Channel                          int        `json:"channel"`
	Destinations                     []string   `json:"destinations"`
	Enabled                          bool       `json:"enabled"`
	Filter                           string     `json:"filter"`
	Floating                         bool       `json:"floating"`
	Hats                             bool       `json:"hats"`
	Name                             string     `json:"name"`
	Polvolt                          bool       `json:"polvolt"`
	Range                            string     `json:"range"`
	Transducer                       Transducer `json:"transducer"`
}

type Transducer struct {
	Requires200V bool   `json:"requires200V"`
	RequiresCcld bool   `json:"requiresCcld"`
	Sensitivity  int    `json:"sensitivity"`
	SerialNumber int    `json:"serialNumber"`
	Type         Type   `json:"type"`
	Unit         string `json:"unit"`
}

type Type struct {
	Model   string `json:"model"`
	Number  string `json:"number"`
	Prefix  string `json:"prefix"`
	Variant string `json:"variant"`
}

// Function to load config from file
func LoadConfig(filename string) (*LanxiConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config LanxiConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &config, nil
}
