package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 10. SAUVEGARDE & RÉCUPÉRATION
// ============================================================================

// BackupType type de sauvegarde
type BackupType string

const (
	BackupFull         BackupType = "FULL"
	BackupIncremental  BackupType = "INCREMENTAL"
	BackupDifferential BackupType = "DIFFERENTIAL"
)

// BackupConfig configuration de sauvegarde
type BackupConfig struct {
	Enabled          bool          `json:"enabled"`
	AutoBackup       bool          `json:"auto_backup"`
	BackupInterval   time.Duration `json:"backup_interval"`
	BackupPath       string        `json:"backup_path"`
	MaxBackups       int           `json:"max_backups"`
	CompressionLevel int           `json:"compression_level"`
	EncryptBackups   bool          `json:"encrypt_backups"`
	IncludeHidden    bool          `json:"include_hidden"`
	ExcludePatterns  []string      `json:"exclude_patterns"`
}

// NewBackupConfig crée une config par défaut
func NewBackupConfig() *BackupConfig {
	return &BackupConfig{
		Enabled:          true,
		AutoBackup:       false,
		BackupInterval:   24 * time.Hour,
		BackupPath:       "backups",
		MaxBackups:       10,
		CompressionLevel: 6,
		EncryptBackups:   false,
		IncludeHidden:    false,
		ExcludePatterns:  []string{"*.tmp", "*.log", "*.bak", "Thumbs.db", ".DS_Store"},
	}
}

// BackupInfo informations sur une sauvegarde
type BackupInfo struct {
	ID          string     `json:"id"`
	Type        BackupType `json:"type"`
	CreatedAt   time.Time  `json:"created_at"`
	SourcePath  string     `json:"source_path"`
	BackupPath  string     `json:"backup_path"`
	Size        int64      `json:"size"`
	FileCount   int        `json:"file_count"`
	Compressed  bool       `json:"compressed"`
	Encrypted   bool       `json:"encrypted"`
	BaseBackup  string     `json:"base_backup,omitempty"`
	Description string     `json:"description,omitempty"`
	Checksum    string     `json:"checksum,omitempty"`
}

// BackupManager gère les sauvegardes
type BackupManager struct {
	config     *BackupConfig
	backups    []*BackupInfo
	mu         sync.RWMutex
	stopChan   chan bool
	running    bool
	lastBackup time.Time
	
	// États des fichiers pour incrémentiel
	fileStates map[string]*FileBackupState
}

// FileBackupState état d'un fichier
type FileBackupState struct {
	Path    string    `json:"path"`
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
	Hash    string    `json:"hash,omitempty"`
}

// NewBackupManager crée un gestionnaire de backups
func NewBackupManager(config *BackupConfig) *BackupManager {
	bm := &BackupManager{
		config:     config,
		backups:    make([]*BackupInfo, 0),
		stopChan:   make(chan bool),
		fileStates: make(map[string]*FileBackupState),
	}
	
	bm.loadMetadata()
	
	return bm
}

// StartAutoBackup démarre les backups automatiques
func (bm *BackupManager) StartAutoBackup() {
	bm.mu.Lock()
	if bm.running || !bm.config.AutoBackup {
		bm.mu.Unlock()
		return
	}
	bm.running = true
	bm.stopChan = make(chan bool)
	bm.mu.Unlock()
	
	go func() {
		ticker := time.NewTicker(bm.config.BackupInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-bm.stopChan:
				return
			case <-ticker.C:
				sourcePath := getExecutableDir()
				bm.CreateBackup(sourcePath, BackupIncremental, "Auto backup")
			}
		}
	}()
	
	addLog("🔄 Sauvegarde automatique activée")
}

// StopAutoBackup arrête les backups automatiques
func (bm *BackupManager) StopAutoBackup() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	if bm.running {
		close(bm.stopChan)
		bm.running = false
	}
}

// CreateBackup crée une sauvegarde
func (bm *BackupManager) CreateBackup(sourcePath string, backupType BackupType, description string) (*BackupInfo, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	// Créer le répertoire
	backupDir := filepath.Join(getExecutableDir(), bm.config.BackupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, err
	}
	
	// Nom du fichier
	timestamp := time.Now().Format("20060102_150405")
	backupID := fmt.Sprintf("backup_%s_%s", timestamp, strings.ToLower(string(backupType)))
	backupFile := filepath.Join(backupDir, backupID+".zip")
	
	// Créer le ZIP
	zipFile, err := os.Create(backupFile)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()
	
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	
	// Collecter les fichiers
	var filesToBackup []string
	var totalSize int64
	
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Ignorer le dossier de backup lui-même
		if strings.HasPrefix(path, backupDir) {
			return nil
		}
		
		// Fichiers cachés
		if !bm.config.IncludeHidden && strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		
		// Patterns d'exclusion
		for _, pattern := range bm.config.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil
			}
		}
		
		// Pour backup incrémentiel
		if backupType == BackupIncremental {
			relPath, _ := filepath.Rel(sourcePath, path)
			if state, exists := bm.fileStates[relPath]; exists {
				if state.ModTime.Equal(info.ModTime()) && state.Size == info.Size() {
					return nil // Fichier inchangé
				}
			}
		}
		
		filesToBackup = append(filesToBackup, path)
		totalSize += info.Size()
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Ajouter les fichiers au ZIP
	for _, filePath := range filesToBackup {
		relPath, _ := filepath.Rel(sourcePath, filePath)
		
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			continue
		}
		
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		
		io.Copy(writer, file)
		file.Close()
		
		// Mettre à jour l'état
		info, _ := os.Stat(filePath)
		if info != nil {
			bm.fileStates[relPath] = &FileBackupState{
				Path:    relPath,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}
		}
	}
	
	zipWriter.Close()
	zipFile.Close()
	
	// Taille finale
	finalInfo, _ := os.Stat(backupFile)
	var finalSize int64
	if finalInfo != nil {
		finalSize = finalInfo.Size()
	}
	
	// Info backup
	backup := &BackupInfo{
		ID:          backupID,
		Type:        backupType,
		CreatedAt:   time.Now(),
		SourcePath:  sourcePath,
		BackupPath:  backupFile,
		Size:        finalSize,
		FileCount:   len(filesToBackup),
		Compressed:  true,
		Encrypted:   bm.config.EncryptBackups,
		Description: description,
	}
	
	bm.backups = append(bm.backups, backup)
	bm.lastBackup = time.Now()
	
	bm.rotateBackups()
	bm.saveMetadata()
	
	addLog(fmt.Sprintf("✅ Backup créé: %s (%d fichiers, %s)", backupID, len(filesToBackup), FormatFileSize(finalSize)))
	
	return backup, nil
}

// rotateBackups supprime les anciens backups
func (bm *BackupManager) rotateBackups() {
	if len(bm.backups) <= bm.config.MaxBackups {
		return
	}
	
	// Trier par date
	sort.Slice(bm.backups, func(i, j int) bool {
		return bm.backups[i].CreatedAt.Before(bm.backups[j].CreatedAt)
	})
	
	// Supprimer les plus anciens
	toRemove := len(bm.backups) - bm.config.MaxBackups
	for i := 0; i < toRemove; i++ {
		os.Remove(bm.backups[i].BackupPath)
		addLog(fmt.Sprintf("🗑️ Ancien backup supprimé: %s", bm.backups[i].ID))
	}
	
	bm.backups = bm.backups[toRemove:]
}

func (bm *BackupManager) saveMetadata() {
	backupDir := filepath.Join(getExecutableDir(), bm.config.BackupPath)
	metaPath := filepath.Join(backupDir, "backups.json")
	
	data, err := json.MarshalIndent(bm.backups, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(metaPath, data, 0644)
}

func (bm *BackupManager) loadMetadata() {
	backupDir := filepath.Join(getExecutableDir(), bm.config.BackupPath)
	metaPath := filepath.Join(backupDir, "backups.json")
	
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}
	
	json.Unmarshal(data, &bm.backups)
}

// GetBackups retourne la liste des backups
func (bm *BackupManager) GetBackups() []*BackupInfo {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	
	result := make([]*BackupInfo, len(bm.backups))
	copy(result, bm.backups)
	return result
}

// RestoreBackup restaure une sauvegarde
func (bm *BackupManager) RestoreBackup(backupID, destPath string) error {
	bm.mu.RLock()
	var backup *BackupInfo
	for _, b := range bm.backups {
		if b.ID == backupID {
			backup = b
			break
		}
	}
	bm.mu.RUnlock()
	
	if backup == nil {
		return fmt.Errorf("backup non trouvé: %s", backupID)
	}
	
	// Ouvrir le ZIP
	reader, err := zip.OpenReader(backup.BackupPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	
	// Extraire
	for _, file := range reader.File {
		destFile := filepath.Join(destPath, file.Name)
		
		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			continue
		}
		
		if file.FileInfo().IsDir() {
			os.MkdirAll(destFile, 0755)
			continue
		}
		
		srcFile, err := file.Open()
		if err != nil {
			continue
		}
		
		dstFile, err := os.Create(destFile)
		if err != nil {
			srcFile.Close()
			continue
		}
		
		io.Copy(dstFile, srcFile)
		
		srcFile.Close()
		dstFile.Close()
	}
	
	addLog(fmt.Sprintf("✅ Backup restauré: %s -> %s", backupID, destPath))
	
	return nil
}

// DeleteBackup supprime un backup
func (bm *BackupManager) DeleteBackup(backupID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	for i, b := range bm.backups {
		if b.ID == backupID {
			os.Remove(b.BackupPath)
			bm.backups = append(bm.backups[:i], bm.backups[i+1:]...)
			bm.saveMetadata()
			return nil
		}
	}
	
	return fmt.Errorf("backup non trouvé: %s", backupID)
}

// ============================================================================
// 10.4 SNAPSHOTS
// ============================================================================

// Snapshot représente un snapshot
type Snapshot struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	CreatedAt   time.Time            `json:"created_at"`
	SourcePath  string               `json:"source_path"`
	FileCount   int                  `json:"file_count"`
	TotalSize   int64                `json:"total_size"`
	Files       map[string]*FileSnap `json:"files"`
	Description string               `json:"description"`
}

// FileSnap état d'un fichier
type FileSnap struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Hash    string    `json:"hash,omitempty"`
	IsDir   bool      `json:"is_dir"`
}

// SnapshotManager gère les snapshots
type SnapshotManager struct {
	snapshots    map[string]*Snapshot
	mu           sync.RWMutex
	snapshotPath string
	maxSnapshots int
}

// NewSnapshotManager crée un gestionnaire de snapshots
func NewSnapshotManager(snapshotPath string, maxSnapshots int) *SnapshotManager {
	sm := &SnapshotManager{
		snapshots:    make(map[string]*Snapshot),
		snapshotPath: snapshotPath,
		maxSnapshots: maxSnapshots,
	}
	
	os.MkdirAll(snapshotPath, 0755)
	sm.loadSnapshots()
	
	return sm
}

// CreateSnapshot crée un snapshot
func (sm *SnapshotManager) CreateSnapshot(sourcePath, name, description string) (*Snapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	timestamp := time.Now().Format("20060102_150405")
	snapshotID := fmt.Sprintf("snap_%s", timestamp)
	
	snapshot := &Snapshot{
		ID:          snapshotID,
		Name:        name,
		CreatedAt:   time.Now(),
		SourcePath:  sourcePath,
		Files:       make(map[string]*FileSnap),
		Description: description,
	}
	
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		relPath, _ := filepath.Rel(sourcePath, path)
		
		fileSnap := &FileSnap{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}
		
		if !info.IsDir() && info.Size() < 50*1024*1024 {
			if hash, err := StreamHash(path); err == nil {
				fileSnap.Hash = hash
			}
			snapshot.TotalSize += info.Size()
			snapshot.FileCount++
		}
		
		snapshot.Files[relPath] = fileSnap
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	sm.snapshots[snapshotID] = snapshot
	sm.saveSnapshot(snapshot)
	sm.rotateSnapshots()
	
	addLog(fmt.Sprintf("📸 Snapshot créé: %s (%d fichiers)", name, snapshot.FileCount))
	
	return snapshot, nil
}

// GetSnapshot retourne un snapshot
func (sm *SnapshotManager) GetSnapshot(id string) (*Snapshot, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	snap, ok := sm.snapshots[id]
	return snap, ok
}

// GetSnapshots retourne tous les snapshots
func (sm *SnapshotManager) GetSnapshots() []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	var result []*Snapshot
	for _, snap := range sm.snapshots {
		result = append(result, snap)
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	
	return result
}

// CompareSnapshots compare deux snapshots
func (sm *SnapshotManager) CompareSnapshots(id1, id2 string) (*SnapshotDiff, error) {
	sm.mu.RLock()
	snap1, ok1 := sm.snapshots[id1]
	snap2, ok2 := sm.snapshots[id2]
	sm.mu.RUnlock()
	
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("snapshot non trouvé")
	}
	
	diff := &SnapshotDiff{
		Snapshot1: id1,
		Snapshot2: id2,
		Added:     make([]string, 0),
		Removed:   make([]string, 0),
		Modified:  make([]string, 0),
	}
	
	for path, file1 := range snap1.Files {
		if file2, exists := snap2.Files[path]; exists {
			if file1.Hash != file2.Hash || file1.Size != file2.Size {
				diff.Modified = append(diff.Modified, path)
			}
		} else {
			diff.Removed = append(diff.Removed, path)
		}
	}
	
	for path := range snap2.Files {
		if _, exists := snap1.Files[path]; !exists {
			diff.Added = append(diff.Added, path)
		}
	}
	
	return diff, nil
}

// SnapshotDiff différences entre snapshots
type SnapshotDiff struct {
	Snapshot1 string
	Snapshot2 string
	Added     []string
	Removed   []string
	Modified  []string
}

// DeleteSnapshot supprime un snapshot
func (sm *SnapshotManager) DeleteSnapshot(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if _, exists := sm.snapshots[id]; !exists {
		return fmt.Errorf("snapshot non trouvé: %s", id)
	}
	
	snapFile := filepath.Join(sm.snapshotPath, id+".json")
	os.Remove(snapFile)
	
	delete(sm.snapshots, id)
	
	return nil
}

func (sm *SnapshotManager) saveSnapshot(snap *Snapshot) {
	snapFile := filepath.Join(sm.snapshotPath, snap.ID+".json")
	data, _ := json.MarshalIndent(snap, "", "  ")
	os.WriteFile(snapFile, data, 0644)
}

func (sm *SnapshotManager) loadSnapshots() {
	entries, err := os.ReadDir(sm.snapshotPath)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			snapFile := filepath.Join(sm.snapshotPath, entry.Name())
			data, err := os.ReadFile(snapFile)
			if err != nil {
				continue
			}
			
			var snap Snapshot
			if err := json.Unmarshal(data, &snap); err == nil {
				sm.snapshots[snap.ID] = &snap
			}
		}
	}
}

func (sm *SnapshotManager) rotateSnapshots() {
	if len(sm.snapshots) <= sm.maxSnapshots {
		return
	}
	
	var oldest *Snapshot
	for _, snap := range sm.snapshots {
		if oldest == nil || snap.CreatedAt.Before(oldest.CreatedAt) {
			oldest = snap
		}
	}
	
	if oldest != nil {
		sm.DeleteSnapshot(oldest.ID)
	}
}

// ============================================================================
// GLOBAL INSTANCES
// ============================================================================

var (
	globalBackupConfig  = NewBackupConfig()
	globalBackupManager *BackupManager
)

func init() {
	globalBackupManager = NewBackupManager(globalBackupConfig)
}

// GetBackupConfig retourne la config
func GetBackupConfig() *BackupConfig { return globalBackupConfig }

// GetBackupManager retourne le gestionnaire
func GetBackupManager() *BackupManager { return globalBackupManager }
