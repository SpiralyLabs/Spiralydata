package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (c *Client) PullAllFromServer() {
	if c.isProcessing {
		addLog("⏳ Opération en cours...")
		return
	}
	
	c.isProcessing = true
	time.Sleep(200 * time.Millisecond)
	
	c.pendingMu.Lock()
	pendingCount := len(c.pendingChanges)
	if pendingCount > 0 {
		addLog(fmt.Sprintf("📦 Traitement de %d changements en attente...", pendingCount))
		applied := 0
		skipped := 0
		
		for _, change := range c.pendingChanges {
			if c.shouldApplyChange(change) {
				c.applyChange(change)
				applied++
			} else {
				skipped++
			}
			
			// Petit délai pour éviter la saturation
			if applied > 0 && applied%20 == 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		c.pendingChanges = []FileChange{}
		
		if skipped > 0 {
			addLog(fmt.Sprintf("⏭️ %d fichiers ignorés (déjà à jour)", skipped))
		}
		if applied > 0 {
			addLog(fmt.Sprintf("✅ %d fichiers appliqués", applied))
		}
	} else {
		addLog("ℹ️ Aucun changement en attente")
	}
	c.pendingMu.Unlock()
	
	time.Sleep(300 * time.Millisecond)
	
	addLog("🔄 Demande des fichiers au serveur...")
	
	reqMsg := map[string]string{
		"type":   "request_all_files",
		"origin": "client",
	}
	
	if err := c.ws.WriteJSON(reqMsg); err != nil {
		addLog("❌ Erreur envoi")
		c.isProcessing = false
		return
	}
	
	go func() {
		time.Sleep(4 * time.Second)
		c.isProcessing = false
		addLog("✅ Réception terminée")
	}()
}

// Vérifie si un changement doit être appliqué (compare les hash)
func (c *Client) shouldApplyChange(change FileChange) bool {
	if change.IsDir {
		return true // Toujours créer les dossiers
	}
	
	if change.Op == "remove" {
		return true // Toujours supprimer
	}
	
	// Vérifier si le fichier existe localement
	normalizedPath := filepath.FromSlash(change.FileName)
	localPath := filepath.Join(c.localDir, normalizedPath)
	
	localData, err := os.ReadFile(localPath)
	if err != nil {
		// Fichier n'existe pas localement, on doit l'appliquer
		return true
	}
	
	// Calculer le hash du fichier local
	localHash := sha256.Sum256(localData)
	
	// Décoder le contenu du serveur
	serverData, err := base64.StdEncoding.DecodeString(change.Content)
	if err != nil {
		return true // En cas d'erreur, appliquer quand même
	}
	
	// Calculer le hash du fichier serveur
	serverHash := sha256.Sum256(serverData)
	
	// Comparer les hash
	return localHash != serverHash
}

func (c *Client) PushLocalChanges() {
	if c.isProcessing {
		addLog("⏳ Opération en cours...")
		return
	}
	
	c.isProcessing = true
	addLog("🔄 Envoi au serveur...")
	
	time.Sleep(200 * time.Millisecond)
	
	allFiles := make(map[string]time.Time)
	allDirs := make(map[string]time.Time)
	c.scanCurrentState(c.localDir, "", allFiles, allDirs)
	
	sent := 0
	
	// Supprimer les dossiers et fichiers manquants
	c.mu.Lock()
	for knownDir := range c.knownDirs {
		if _, exists := allDirs[knownDir]; !exists {
			change := FileChange{
				FileName: knownDir,
				Op:       "remove",
				IsDir:    true,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			delete(c.knownDirs, knownDir)
			sent++
			time.Sleep(150 * time.Millisecond)
		}
	}
	
	for knownFile := range c.knownFiles {
		if _, exists := allFiles[knownFile]; !exists {
			change := FileChange{
				FileName: knownFile,
				Op:       "remove",
				IsDir:    false,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			delete(c.knownFiles, knownFile)
			sent++
			time.Sleep(150 * time.Millisecond)
		}
	}
	c.mu.Unlock()
	
	// Créer dossiers et fichiers
	for i, dirPath := range getSortedKeys(allDirs) {
		c.mu.Lock()
		_, known := c.knownDirs[dirPath]
		c.mu.Unlock()
		
		if !known {
			change := FileChange{
				FileName: dirPath,
				Op:       "mkdir",
				IsDir:    true,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			c.mu.Lock()
			c.knownDirs[dirPath] = time.Now()
			c.mu.Unlock()
			sent++
			time.Sleep(150 * time.Millisecond)
			
			if i > 0 && i%5 == 0 {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	
	for i, filePath := range getSortedKeys(allFiles) {
		modTime := allFiles[filePath]
		
		c.mu.Lock()
		lastMod, known := c.knownFiles[filePath]
		shouldSend := !known || modTime.After(lastMod)
		c.mu.Unlock()
		
		if shouldSend {
			fullPath := filepath.Join(c.localDir, filepath.FromSlash(filePath))
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			
			change := FileChange{
				FileName: filePath,
				Op:       "write",
				Content:  base64.StdEncoding.EncodeToString(data),
				IsDir:    false,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			c.mu.Lock()
			c.knownFiles[filePath] = modTime
			c.mu.Unlock()
			sent++
			time.Sleep(150 * time.Millisecond)
			
			if i > 0 && i%10 == 0 {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	
	time.Sleep(300 * time.Millisecond)
	
	c.isProcessing = false
	addLog(fmt.Sprintf("✅ %d opérations envoyées", sent))
}

func (c *Client) ClearLocalFiles() {
	if c.isProcessing {
		addLog("⏳ Opération en cours...")
		return
	}
	
	if c.autoSync {
		addLog("⚠️ Désactivez la sync auto d'abord")
		return
	}
	
	c.isProcessing = true
	defer func() { c.isProcessing = false }()
	
	time.Sleep(300 * time.Millisecond)
	
	addLog("🗑️ Suppression fichiers locaux...")
	
	entries, err := os.ReadDir(c.localDir)
	if err != nil {
		addLog("❌ Erreur lecture dossier")
		return
	}
	
	count := 0
	for i, entry := range entries {
		path := filepath.Join(c.localDir, entry.Name())
		if err := os.RemoveAll(path); err == nil {
			count++
		}
		time.Sleep(200 * time.Millisecond)
		
		if i > 0 && i%5 == 0 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	
	c.mu.Lock()
	c.knownFiles = make(map[string]time.Time)
	c.knownDirs = make(map[string]time.Time)
	c.lastState = make(map[string]time.Time)
	c.lastDirs = make(map[string]time.Time)
	c.mu.Unlock()
	
	time.Sleep(300 * time.Millisecond)
	
	addLog(fmt.Sprintf("✅ %d éléments supprimés", count))
}

func (c *Client) watchRecursive() {
	if c.watcherActive {
		return
	}
	c.watcherActive = true
	
	time.Sleep(300 * time.Millisecond)
	
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		addLog(fmt.Sprintf("❌ Erreur watcher: %v", err))
		return
	}
	defer watcher.Close()

	c.addDirToWatcher(watcher, c.localDir)
	addLog("👀 Surveillance activée")

	for {
		if c.shouldExit {
			return
		}
		
		select {
		case event := <-watcher.Events:
			if c.autoSync {
				time.Sleep(100 * time.Millisecond)
				c.handleLocalEvent(event)
			}
			
			if event.Op&fsnotify.Create != 0 {
				time.Sleep(150 * time.Millisecond)
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}
		case err := <-watcher.Errors:
			if !c.shouldExit {
				addLog(fmt.Sprintf("⚠️ Erreur watcher: %v", err))
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (c *Client) handleLocalEvent(event fsnotify.Event) {
	relPath, err := filepath.Rel(c.localDir, event.Name)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)
	
	c.mu.Lock()
	if until, exists := c.skipNext[relPath]; exists && time.Now().Before(until) {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	
	time.Sleep(100 * time.Millisecond)
	
	if event.Op&fsnotify.Create != 0 || event.Op&fsnotify.Write != 0 {
		info, err := os.Stat(event.Name)
		if err != nil {
			return
		}
		
		if info.IsDir() {
			change := FileChange{
				FileName: relPath,
				Op:       "mkdir",
				IsDir:    true,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			// Pas de log individuel
		} else {
			time.Sleep(100 * time.Millisecond)
			data, err := os.ReadFile(event.Name)
			if err != nil {
				return
			}
			change := FileChange{
				FileName: relPath,
				Op:       "write",
				Content:  base64.StdEncoding.EncodeToString(data),
				IsDir:    false,
				Origin:   "client",
			}
			c.ws.WriteJSON(change)
			// Pas de log individuel
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (c *Client) addDirToWatcher(watcher *fsnotify.Watcher, dir string) {
	watcher.Add(dir)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	
	for i, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			c.addDirToWatcher(watcher, subDir)
			
			if i > 0 && i%5 == 0 {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

func (c *Client) periodicScanner() {
	if c.scanRunning || !c.autoSync {
		return
	}
	c.scanRunning = true
	defer func() { c.scanRunning = false }()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if c.shouldExit || !c.autoSync {
			return
		}

		time.Sleep(200 * time.Millisecond)

		currentFiles := make(map[string]time.Time)
		currentDirs := make(map[string]time.Time)
		c.scanCurrentState(c.localDir, "", currentFiles, currentDirs)

		c.mu.Lock()
		
		// Compteurs pour les logs groupés
		dirsRemoved := 0
		dirsCreated := 0
		filesModified := 0
		filesRemoved := 0
		
		for oldDir := range c.lastDirs {
			if until, exists := c.skipNext[oldDir]; exists && time.Now().Before(until) {
				continue
			}
			if _, exists := currentDirs[oldDir]; !exists {
				change := FileChange{
					FileName: oldDir,
					Op:       "remove",
					IsDir:    true,
					Origin:   "client",
				}
				c.ws.WriteJSON(change)
				delete(c.knownDirs, oldDir)
				dirsRemoved++
				time.Sleep(150 * time.Millisecond)
			}
		}

		for newDir, modTime := range currentDirs {
			if _, known := c.lastDirs[newDir]; !known {
				if until, exists := c.skipNext[newDir]; exists && time.Now().Before(until) {
					continue
				}
				change := FileChange{
					FileName: newDir,
					Op:       "mkdir",
					IsDir:    true,
					Origin:   "client",
				}
				c.ws.WriteJSON(change)
				c.knownDirs[newDir] = modTime
				dirsCreated++
				time.Sleep(150 * time.Millisecond)
			}
		}

		for name, modTime := range currentFiles {
			if until, exists := c.skipNext[name]; exists && time.Now().Before(until) {
				continue
			}

			lastMod, known := c.lastState[name]
			if !known || modTime.After(lastMod) {
				time.Sleep(50 * time.Millisecond)
				c.sendFileNow(name)
				filesModified++
			}
		}

		for oldFile := range c.lastState {
			if _, still := currentFiles[oldFile]; !still {
				if until, exists := c.skipNext[oldFile]; exists && time.Now().Before(until) {
					delete(c.skipNext, oldFile)
					continue
				}
				
				change := FileChange{
					FileName: oldFile,
					Op:       "remove",
					IsDir:    false,
					Origin:   "client",
				}
				c.ws.WriteJSON(change)
				delete(c.knownFiles, oldFile)
				filesRemoved++
				time.Sleep(150 * time.Millisecond)
			}
		}

		c.lastState = currentFiles
		c.lastDirs = currentDirs
		c.mu.Unlock()
		
		// Log groupé uniquement s'il y a des changements
		if dirsRemoved > 0 || dirsCreated > 0 || filesModified > 0 || filesRemoved > 0 {
			var changes []string
			if dirsCreated > 0 {
				changes = append(changes, fmt.Sprintf("%d dossiers créés", dirsCreated))
			}
			if dirsRemoved > 0 {
				changes = append(changes, fmt.Sprintf("%d dossiers supprimés", dirsRemoved))
			}
			if filesModified > 0 {
				changes = append(changes, fmt.Sprintf("%d fichiers modifiés", filesModified))
			}
			if filesRemoved > 0 {
				changes = append(changes, fmt.Sprintf("%d fichiers supprimés", filesRemoved))
			}
			addLog(fmt.Sprintf("📤 Sync: %s", strings.Join(changes, ", ")))
		}
	}
}

func (c *Client) scanCurrentState(basePath, relPath string, files map[string]time.Time, dirs map[string]time.Time) {
	fullPath := filepath.Join(basePath, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return
	}

	for i, entry := range entries {
		itemRelPath := filepath.ToSlash(filepath.Join(relPath, entry.Name()))
		info, _ := entry.Info()
		
		if entry.IsDir() {
			if info != nil {
				dirs[itemRelPath] = info.ModTime()
			}
			c.scanCurrentState(basePath, filepath.Join(relPath, entry.Name()), files, dirs)
		} else {
			if info != nil {
				files[itemRelPath] = info.ModTime()
			}
		}
		
		if i > 0 && i%20 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func (c *Client) sendFileNow(relPath string) {
	fullPath := filepath.Join(c.localDir, filepath.FromSlash(relPath))
	
	time.Sleep(50 * time.Millisecond)
	
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return
	}
	
	change := FileChange{
		FileName: relPath,
		Op:       "write",
		Content:  base64.StdEncoding.EncodeToString(data),
		IsDir:    false,
		Origin:   "client",
	}
	
	c.ws.WriteJSON(change)
	c.knownFiles[relPath] = time.Now()
	time.Sleep(150 * time.Millisecond)
}