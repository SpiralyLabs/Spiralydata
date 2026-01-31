package main

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 8.1 SYSTÈME DE LOGS AVANCÉ
// ============================================================================

// LogLevel niveau de log
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarning
	LogError
	LogCritical
)

// String retourne le nom du niveau
func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarning:
		return "WARNING"
	case LogError:
		return "ERROR"
	case LogCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Icon retourne l'icône du niveau
func (l LogLevel) Icon() string {
	switch l {
	case LogDebug:
		return "🔍"
	case LogInfo:
		return "ℹ️"
	case LogWarning:
		return "⚠️"
	case LogError:
		return "❌"
	case LogCritical:
		return "🚨"
	default:
		return "📝"
	}
}

// ParseLogLevel parse un niveau depuis une string
func ParseLogLevel(s string) LogLevel {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LogDebug
	case "INFO":
		return LogInfo
	case "WARNING", "WARN":
		return LogWarning
	case "ERROR":
		return LogError
	case "CRITICAL", "FATAL":
		return LogCritical
	default:
		return LogInfo
	}
}

// LogEntry représente une entrée de log structurée
type LogEntry struct {
	ID         int64             `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Level      LogLevel          `json:"level"`
	LevelStr   string            `json:"level_str"`
	Category   string            `json:"category"`
	Message    string            `json:"message"`
	Context    map[string]string `json:"context,omitempty"`
	UserID     string            `json:"user_id,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	ClientIP   string            `json:"client_ip,omitempty"`
	FilePath   string            `json:"file_path,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	Error      string            `json:"error,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
}

// Format formate l'entrée pour affichage console
func (e *LogEntry) Format() string {
	ts := e.Timestamp.Format("15:04:05")
	return fmt.Sprintf("%s %s [%s] %s", ts, e.Level.Icon(), e.Category, e.Message)
}

// FormatFull formate l'entrée avec tous les détails
func (e *LogEntry) FormatFull() string {
	ts := e.Timestamp.Format("2006-01-02 15:04:05.000")
	result := fmt.Sprintf("%s | %-8s | %-12s | %s", ts, e.Level.String(), e.Category, e.Message)
	
	if e.UserID != "" {
		result += fmt.Sprintf(" | user=%s", e.UserID)
	}
	if e.ClientIP != "" {
		result += fmt.Sprintf(" | ip=%s", e.ClientIP)
	}
	if e.Duration > 0 {
		result += fmt.Sprintf(" | dur=%v", e.Duration)
	}
	if e.Error != "" {
		result += fmt.Sprintf(" | err=%s", e.Error)
	}
	
	return result
}

// ToJSON convertit en JSON
func (e *LogEntry) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// ============================================================================
// ADVANCED LOGGER
// ============================================================================

// AdvancedLogger logger avancé avec toutes les fonctionnalités
type AdvancedLogger struct {
	entries      []*LogEntry
	mu           sync.RWMutex
	maxEntries   int
	minLevel     LogLevel
	
	// Fichier
	logDir       string
	logFile      string
	fileHandle   *os.File
	currentSize  int64
	
	// Rotation
	maxFileSize  int64
	maxFiles     int
	compressOld  bool
	
	// Compteur
	entryCounter int64
	
	// Callbacks
	onLog        []func(*LogEntry)
	onError      []func(*LogEntry)
}

// NewAdvancedLogger crée un nouveau logger
func NewAdvancedLogger(maxEntries int) *AdvancedLogger {
	return &AdvancedLogger{
		entries:     make([]*LogEntry, 0),
		maxEntries:  maxEntries,
		minLevel:    LogInfo,
		maxFileSize: 10 * 1024 * 1024, // 10 MB
		maxFiles:    5,
		compressOld: true,
	}
}

// SetLevel définit le niveau minimum
func (l *AdvancedLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// GetLevel retourne le niveau minimum
func (l *AdvancedLogger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.minLevel
}

// SetLogDir définit le répertoire de logs
func (l *AdvancedLogger) SetLogDir(dir string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	l.logDir = dir
	l.logFile = filepath.Join(dir, "spiraly.log")
	
	return l.openLogFile()
}

func (l *AdvancedLogger) openLogFile() error {
	if l.fileHandle != nil {
		l.fileHandle.Close()
	}
	
	file, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	
	l.fileHandle = file
	
	info, _ := file.Stat()
	if info != nil {
		l.currentSize = info.Size()
	}
	
	return nil
}

// Log ajoute une entrée
func (l *AdvancedLogger) Log(level LogLevel, category, message string) *LogEntry {
	return l.LogWithContext(level, category, message, nil)
}

// LogWithContext ajoute une entrée avec contexte
func (l *AdvancedLogger) LogWithContext(level LogLevel, category, message string, ctx map[string]string) *LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if level < l.minLevel {
		return nil
	}
	
	l.entryCounter++
	entry := &LogEntry{
		ID:        l.entryCounter,
		Timestamp: time.Now(),
		Level:     level,
		LevelStr:  level.String(),
		Category:  category,
		Message:   message,
		Context:   ctx,
	}
	
	// Ajouter stack trace pour erreurs critiques
	if level >= LogError {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		entry.StackTrace = string(buf[:n])
	}
	
	// Ajouter à la liste
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[1:]
	}
	
	// Écrire dans le fichier
	if l.fileHandle != nil {
		line := entry.FormatFull() + "\n"
		l.fileHandle.WriteString(line)
		l.currentSize += int64(len(line))
		
		if l.currentSize >= l.maxFileSize {
			l.rotate()
		}
	}
	
	// Callbacks
	for _, cb := range l.onLog {
		go cb(entry)
	}
	
	if level >= LogError {
		for _, cb := range l.onError {
			go cb(entry)
		}
	}
	
	return entry
}

// Méthodes de commodité
func (l *AdvancedLogger) Debug(category, message string) *LogEntry {
	return l.Log(LogDebug, category, message)
}

func (l *AdvancedLogger) Info(category, message string) *LogEntry {
	return l.Log(LogInfo, category, message)
}

func (l *AdvancedLogger) Warning(category, message string) *LogEntry {
	return l.Log(LogWarning, category, message)
}

func (l *AdvancedLogger) Error(category, message string) *LogEntry {
	return l.Log(LogError, category, message)
}

func (l *AdvancedLogger) Critical(category, message string) *LogEntry {
	return l.Log(LogCritical, category, message)
}

// LogError log une erreur avec détails
func (l *AdvancedLogger) LogErr(category string, err error) *LogEntry {
	if err == nil {
		return nil
	}
	entry := l.Log(LogError, category, err.Error())
	if entry != nil {
		entry.Error = err.Error()
	}
	return entry
}

// LogPerformance log une métrique de performance
func (l *AdvancedLogger) LogPerformance(operation string, duration time.Duration, details map[string]string) *LogEntry {
	msg := fmt.Sprintf("%s completed in %v", operation, duration)
	entry := l.LogWithContext(LogInfo, "PERF", msg, details)
	if entry != nil {
		entry.Duration = duration
	}
	return entry
}

// LogTransfer log un transfert de fichier
func (l *AdvancedLogger) LogTransfer(path string, size int64, direction string, duration time.Duration) *LogEntry {
	speed := float64(size) / duration.Seconds()
	msg := fmt.Sprintf("%s %s (%s) - %s/s", direction, filepath.Base(path), FormatFileSize(size), FormatFileSize(int64(speed)))
	
	entry := l.Log(LogInfo, "TRANSFER", msg)
	if entry != nil {
		entry.FilePath = path
		entry.Duration = duration
		entry.Context = map[string]string{
			"size":      fmt.Sprintf("%d", size),
			"direction": direction,
			"speed":     FormatFileSize(int64(speed)) + "/s",
		}
	}
	return entry
}

// ============================================================================
// ROTATION DES LOGS
// ============================================================================

func (l *AdvancedLogger) rotate() {
	if l.fileHandle != nil {
		l.fileHandle.Close()
	}
	
	// Renommer les fichiers existants
	for i := l.maxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.logFile, i)
		newPath := fmt.Sprintf("%s.%d", l.logFile, i+1)
		
		if l.compressOld {
			oldPath += ".gz"
			newPath += ".gz"
		}
		
		os.Rename(oldPath, newPath)
	}
	
	// Supprimer le plus ancien
	oldestPath := fmt.Sprintf("%s.%d", l.logFile, l.maxFiles+1)
	if l.compressOld {
		oldestPath += ".gz"
	}
	os.Remove(oldestPath)
	
	// Compresser le fichier actuel
	if l.compressOld {
		l.compressFile(l.logFile, l.logFile+".1.gz")
		os.Remove(l.logFile)
	} else {
		os.Rename(l.logFile, l.logFile+".1")
	}
	
	// Créer un nouveau fichier
	l.openLogFile()
}

func (l *AdvancedLogger) compressFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()
	
	_, err = io.Copy(gzWriter, srcFile)
	return err
}

// ============================================================================
// FILTRAGE DES LOGS
// ============================================================================

// LogFilter filtre de logs
type LogFilter struct {
	MinLevel   *LogLevel
	MaxLevel   *LogLevel
	Categories []string
	StartTime  *time.Time
	EndTime    *time.Time
	Keyword    string
	UserID     string
	ClientIP   string
	Limit      int
}

// GetEntries retourne les entrées filtrées
func (l *AdvancedLogger) GetEntries(filter *LogFilter) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if filter == nil {
		// Retourner une copie
		result := make([]*LogEntry, len(l.entries))
		copy(result, l.entries)
		return result
	}
	
	var result []*LogEntry
	
	for _, entry := range l.entries {
		// Niveau min
		if filter.MinLevel != nil && entry.Level < *filter.MinLevel {
			continue
		}
		// Niveau max
		if filter.MaxLevel != nil && entry.Level > *filter.MaxLevel {
			continue
		}
		// Catégories
		if len(filter.Categories) > 0 {
			found := false
			for _, cat := range filter.Categories {
				if entry.Category == cat {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Temps
		if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
			continue
		}
		// Mot-clé
		if filter.Keyword != "" {
			kw := strings.ToLower(filter.Keyword)
			if !strings.Contains(strings.ToLower(entry.Message), kw) &&
				!strings.Contains(strings.ToLower(entry.Category), kw) {
				continue
			}
		}
		// UserID
		if filter.UserID != "" && entry.UserID != filter.UserID {
			continue
		}
		// ClientIP
		if filter.ClientIP != "" && entry.ClientIP != filter.ClientIP {
			continue
		}
		
		result = append(result, entry)
	}
	
	// Limite
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[len(result)-filter.Limit:]
	}
	
	return result
}

// Search recherche avec regex
func (l *AdvancedLogger) Search(pattern string, limit int) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	regex, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil
	}
	
	var result []*LogEntry
	
	for i := len(l.entries) - 1; i >= 0 && (limit <= 0 || len(result) < limit); i-- {
		entry := l.entries[i]
		if regex.MatchString(entry.Message) || regex.MatchString(entry.Category) {
			result = append(result, entry)
		}
	}
	
	return result
}

// GetCategories retourne les catégories utilisées
func (l *AdvancedLogger) GetCategories() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	catMap := make(map[string]bool)
	for _, entry := range l.entries {
		catMap[entry.Category] = true
	}
	
	var categories []string
	for cat := range catMap {
		categories = append(categories, cat)
	}
	
	sort.Strings(categories)
	return categories
}

// GetStatistics retourne des statistiques
func (l *AdvancedLogger) GetStatistics() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	stats := make(map[string]interface{})
	
	levelCounts := make(map[string]int)
	categoryCounts := make(map[string]int)
	
	for _, entry := range l.entries {
		levelCounts[entry.Level.String()]++
		categoryCounts[entry.Category]++
	}
	
	stats["total"] = len(l.entries)
	stats["by_level"] = levelCounts
	stats["by_category"] = categoryCounts
	
	if len(l.entries) > 0 {
		stats["oldest"] = l.entries[0].Timestamp
		stats["newest"] = l.entries[len(l.entries)-1].Timestamp
	}
	
	return stats
}

// ============================================================================
// EXPORT DES LOGS
// ============================================================================

// ExportToJSON exporte en JSON
func (l *AdvancedLogger) ExportToJSON(path string, filter *LogFilter) error {
	entries := l.GetEntries(filter)
	
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// ExportToCSV exporte en CSV
func (l *AdvancedLogger) ExportToCSV(path string, filter *LogFilter) error {
	entries := l.GetEntries(filter)
	
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// En-tête
	writer.Write([]string{"ID", "Timestamp", "Level", "Category", "Message", "UserID", "ClientIP", "Duration", "Error"})
	
	for _, entry := range entries {
		writer.Write([]string{
			fmt.Sprintf("%d", entry.ID),
			entry.Timestamp.Format(time.RFC3339),
			entry.Level.String(),
			entry.Category,
			entry.Message,
			entry.UserID,
			entry.ClientIP,
			entry.Duration.String(),
			entry.Error,
		})
	}
	
	return nil
}

// ExportToTXT exporte en texte
func (l *AdvancedLogger) ExportToTXT(path string, filter *LogFilter) error {
	entries := l.GetEntries(filter)
	
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	for _, entry := range entries {
		file.WriteString(entry.FormatFull() + "\n")
	}
	
	return nil
}

// Clear efface les logs en mémoire
func (l *AdvancedLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*LogEntry, 0)
}

// Close ferme le logger
func (l *AdvancedLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.fileHandle != nil {
		l.fileHandle.Close()
		l.fileHandle = nil
	}
}

// OnLog ajoute un callback
func (l *AdvancedLogger) OnLog(cb func(*LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLog = append(l.onLog, cb)
}

// OnError ajoute un callback d'erreur
func (l *AdvancedLogger) OnError(cb func(*LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onError = append(l.onError, cb)
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var globalAdvancedLogger *AdvancedLogger

func init() {
	globalAdvancedLogger = NewAdvancedLogger(5000)
	
	// Configurer le répertoire de logs
	logDir := filepath.Join(getExecutableDir(), "logs")
	globalAdvancedLogger.SetLogDir(logDir)
}

// GetAdvancedLogger retourne le logger global
func GetAdvancedLogger() *AdvancedLogger {
	return globalAdvancedLogger
}

// Fonctions helper globales pour le logging applicatif
func AppLogDebug(category, message string) {
	globalAdvancedLogger.Debug(category, message)
}

func AppLogInfo(category, message string) {
	globalAdvancedLogger.Info(category, message)
}

func AppLogWarning(category, message string) {
	globalAdvancedLogger.Warning(category, message)
}

func AppLogError(category, message string) {
	globalAdvancedLogger.Error(category, message)
}

func AppLogCritical(category, message string) {
	globalAdvancedLogger.Critical(category, message)
}
