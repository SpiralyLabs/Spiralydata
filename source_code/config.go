package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type AppConfig struct {
	ServerIP      string  `json:"server_ip"`
	ServerPort    string  `json:"server_port"`
	HostID        string  `json:"host_id"`
	SyncDirectory string  `json:"sync_directory"`
	SaveConfig    bool    `json:"save_config"`
	AutoConnect   bool    `json:"auto_connect"`
	WindowWidth   float32 `json:"window_width,omitempty"`
	WindowHeight  float32 `json:"window_height,omitempty"`
	DarkTheme     bool    `json:"dark_theme"`
	ShowStatusBar bool    `json:"show_status_bar"`
	LogsMaxCount  int     `json:"logs_max_count,omitempty"`
	// Filtres sauvegardés
	FilterExtensions    []string `json:"filter_extensions,omitempty"`
	FilterMode          int      `json:"filter_mode,omitempty"` // 0=Blacklist, 1=Whitelist
	FilterEnabled       bool     `json:"filter_enabled,omitempty"`
	FilterExcludeFolders []string `json:"filter_exclude_folders,omitempty"`
	FilterMaxSize       int64    `json:"filter_max_size,omitempty"`
	FilterExcludeHidden bool     `json:"filter_exclude_hidden,omitempty"`
	// Limites serveur (Host)
	MaxStorageSize     int64 `json:"max_storage_size,omitempty"`     // Limite totale en bytes (0 = illimité)
	MaxFileSize        int64 `json:"max_file_size,omitempty"`        // Taille max par fichier
	MaxFilesCount      int64 `json:"max_files_count,omitempty"`      // Nombre max de fichiers
	WarnStoragePercent int   `json:"warn_storage_percent,omitempty"` // Alerte à ce % (ex: 80)
}

var configFilePath string

func init() {
	configFilePath = filepath.Join(getExecutableDir(), "spiraly_config.json")
}

func LoadConfig() (*AppConfig, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		// Retourner une config par défaut avec thème sombre
		return &AppConfig{DarkTheme: true, ShowStatusBar: true, LogsMaxCount: 100}, err
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return &AppConfig{DarkTheme: true, ShowStatusBar: true, LogsMaxCount: 100}, err
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

// SaveFiltersToConfig sauvegarde les filtres dans la config
func SaveFiltersToConfig(fc *FilterConfig) error {
	config, _ := LoadConfig()
	if config == nil {
		config = &AppConfig{}
	}

	// Sauvegarder les extensions
	config.FilterExtensions = fc.Filters.Extension.GetExtensions()
	config.FilterMode = int(fc.Filters.Extension.GetMode())
	config.FilterEnabled = fc.Filters.Extension.IsEnabled()

	// Sauvegarder les dossiers exclus
	config.FilterExcludeFolders = fc.Filters.Path.ExcludedFolders
	config.FilterExcludeHidden = fc.Filters.Path.ExcludeHidden

	// Sauvegarder la taille max
	config.FilterMaxSize = fc.Filters.Size.MaxSize

	return SaveConfig(config)
}

// LoadFiltersFromConfig charge les filtres depuis la config
func LoadFiltersFromConfig(fc *FilterConfig) {
	config, err := LoadConfig()
	if err != nil || config == nil {
		return
	}

	// Charger les extensions
	fc.Filters.Extension.Clear()
	for _, ext := range config.FilterExtensions {
		fc.Filters.Extension.AddExtension(ext)
	}
	fc.Filters.Extension.SetMode(FilterMode(config.FilterMode))
	fc.Filters.Extension.SetEnabled(config.FilterEnabled)

	// Charger les dossiers exclus
	if len(config.FilterExcludeFolders) > 0 {
		fc.Filters.Path.ExcludedFolders = config.FilterExcludeFolders
	}
	fc.Filters.Path.ExcludeHidden = config.FilterExcludeHidden

	// Charger la taille max
	if config.FilterMaxSize > 0 {
		fc.Filters.Size.MaxSize = config.FilterMaxSize
		fc.Filters.Size.Enabled = true
	}
}

// SyncConfigFile représente la configuration de synchronisation sauvegardée
type SyncConfigFile struct {
	Mode               int      `json:"sync_mode"`
	CompressionEnabled bool     `json:"compression_enabled"`
	CompressionLevel   int      `json:"compression_level"`
	BandwidthLimit     int64    `json:"bandwidth_limit"`
	RetryCount         int      `json:"retry_count"`
	RetryDelaySeconds  int      `json:"retry_delay_seconds"`
	ScheduleEnabled    bool     `json:"schedule_enabled"`
	ScheduleMinutes    int      `json:"schedule_minutes"`
	ScheduleTimes      []string `json:"schedule_times"`
	PriorityExtensions []string `json:"priority_extensions"`
	ConflictStrategy   int      `json:"conflict_strategy"`
}

var syncConfigFilePath string

func init() {
	syncConfigFilePath = filepath.Join(getExecutableDir(), "spiraly_sync_config.json")
}

// SaveSyncConfigToFile sauvegarde la configuration de synchronisation
func SaveSyncConfigToFile(config *SyncConfig) error {
	fileConfig := &SyncConfigFile{
		Mode:               int(config.Mode),
		CompressionEnabled: config.CompressionEnabled,
		CompressionLevel:   config.CompressionLevel,
		BandwidthLimit:     config.BandwidthLimit,
		RetryCount:         config.RetryCount,
		RetryDelaySeconds:  int(config.RetryDelay.Seconds()),
		ScheduleEnabled:    config.ScheduleEnabled,
		ScheduleMinutes:    int(config.ScheduleInterval.Minutes()),
		ScheduleTimes:      config.ScheduleTimes,
		PriorityExtensions: config.PriorityExtensions,
		ConflictStrategy:   int(config.ConflictStrategy),
	}

	data, err := json.MarshalIndent(fileConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(syncConfigFilePath, data, 0644)
}

// LoadSyncConfigFromFile charge la configuration de synchronisation
func LoadSyncConfigFromFile() *SyncConfig {
	config := NewSyncConfig()

	data, err := os.ReadFile(syncConfigFilePath)
	if err != nil {
		return config
	}

	var fileConfig SyncConfigFile
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return config
	}

	config.Mode = SyncMode(fileConfig.Mode)
	config.CompressionEnabled = fileConfig.CompressionEnabled
	config.CompressionLevel = fileConfig.CompressionLevel
	config.BandwidthLimit = fileConfig.BandwidthLimit
	config.RetryCount = fileConfig.RetryCount
	config.RetryDelay = time.Duration(fileConfig.RetryDelaySeconds) * time.Second
	config.ScheduleEnabled = fileConfig.ScheduleEnabled
	config.ScheduleInterval = time.Duration(fileConfig.ScheduleMinutes) * time.Minute
	config.ScheduleTimes = fileConfig.ScheduleTimes
	config.PriorityExtensions = fileConfig.PriorityExtensions
	config.ConflictStrategy = ConflictStrategy(fileConfig.ConflictStrategy)

	return config
} 