package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	ServerIP      string `json:"server_ip"`
	ServerPort    string `json:"server_port"`
	HostID        string `json:"host_id"`
	SyncDirectory string `json:"sync_directory"`
	SaveConfig    bool   `json:"save_config"`
	AutoConnect   bool   `json:"auto_connect"`
}

var (
	configFilePath string
)

func init() {
	configFilePath = filepath.Join(getExecutableDir(), "spiraly_config.json")
}

func LoadConfig() (*AppConfig, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return &AppConfig{}, err
	}
	
	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return &AppConfig{}, err
	}
	
	return &config, nil
}

func SaveConfig(config *AppConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(configFilePath, data, 0644)
}