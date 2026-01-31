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
	c.skipTracking = true // Ignorer le tracking pendant la réception
	time.Sleep(100 * time.Millisecond)

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

			if applied > 0 && applied%20 == 0 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		c.pendingChanges = []FileChange{}

		if skipped > 0 {
			addLog(fmt.Sprintf("⭐ %d fichiers ignorés (déjà à jour)", skipped))
		}
		if applied > 0 {
			addLog(fmt.Sprintf("✅ %d fichiers appliqués", applied))
		}
	} else {
		addLog("ℹ️ Aucun changement en attente")
	}
	c.pendingMu.Unlock()

	time.Sleep(100 * time.Millisecond)

	addLog("🔄 Demande des fichiers au serveur...")

	reqMsg := map[string]string{
		"type":   "request_all_files",
		"origin": "client",
	}

	if err := c.WriteJSONSafe(reqMsg); err != nil {
		addLog("❌ Erreur envoi")
		c.isProcessing = false
		c.skipTracking = false
		return
	}

	go func() {
		time.Sleep(4 * time.Second)
		c.isProcessing = false
		c.skipTracking = false // Réactiver le tracking après la réception
		addLog("✅ Réception terminée")
	}()
}

func (c *Client) shouldApplyChange(change FileChange) bool {
	if change.IsDir {
		return true
	}
	
	if change.Op == "remove" {
		return true
	}
	
	normalizedPath := filepath.FromSlash(change.FileName)
	localPath := filepath.Join(c.localDir, normalizedPath)
	
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return true
	}
	
	localHash := sha256.Sum256(localData)
	
	serverData, err := base64.StdEncoding.DecodeString(change.Content)
	if err != nil {
		return true
	}
	
	serverHash := sha256.Sum256(serverData)
	
	return localHash != serverHash
}

func (c *Client) PushLocalChanges() {
	if c.isProcessing {
		addLog("⏳ Opération en cours...")
		return
	}

	c.isProcessing = true
	addLog("📤 Analyse des fichiers locaux...")

	// Réinitialiser le compteur de fichiers ignorés
	filterConfig := GetFilterConfig()
	filterConfig.Filters.Extension.ResetIgnoredCount()

	// Pause pour laisser l'UI se rafraîchir
	time.Sleep(100 * time.Millisecond)

	// Scanner l'état actuel
	allFiles := make(map[string]time.Time)
	allDirs := make(map[string]time.Time)
	c.scanCurrentState(c.localDir, "", allFiles, allDirs)

	// Filtrer les fichiers et dossiers
	filteredFiles := make(map[string]time.Time)
	filteredDirs := make(map[string]time.Time)
	filteredCount := 0

	for dirPath, modTime := range allDirs {
		if !filterConfig.Filters.Path.ShouldFilter(dirPath) {
			filteredDirs[dirPath] = modTime
		} else {
			filteredCount++
		}
	}

	for filePath, modTime := range allFiles {
		fullPath := filepath.Join(c.localDir, filepath.FromSlash(filePath))
		info, err := os.Stat(fullPath)
		size := int64(0)
		if err == nil {
			size = info.Size()
		}

		if !filterConfig.ShouldFilterFile(filePath, size, false) {
			filteredFiles[filePath] = modTime
		} else {
			filteredCount++
		}
	}

	if filteredCount > 0 {
		addLog(fmt.Sprintf("🔍 %d fichiers/dossiers ignorés (filtres actifs)", filteredCount))
	}

	// Demander l'état du serveur pour comparaison
	serverFiles, serverDirs := c.getServerState()

	var newFiles []string
	var modifiedFiles []string
	var newDirs []string
	var deletedFiles []string
	var deletedDirs []string

	// Détecter les nouveaux dossiers et dossiers supprimés
	for dirPath := range filteredDirs {
		if _, existsOnServer := serverDirs[dirPath]; !existsOnServer {
			newDirs = append(newDirs, dirPath)
		}
	}

	// Détecter les dossiers supprimés localement
	c.mu.Lock()
	for knownDir := range c.knownDirs {
		if _, exists := filteredDirs[knownDir]; !exists {
			deletedDirs = append(deletedDirs, knownDir)
		}
	}
	c.mu.Unlock()

	// Détecter les nouveaux fichiers et fichiers modifiés
	for filePath, modTime := range filteredFiles {
		c.mu.Lock()
		lastMod, known := c.knownFiles[filePath]
		c.mu.Unlock()

		_, existsOnServer := serverFiles[filePath]

		if !existsOnServer && !known {
			// Nouveau fichier qui n'existe pas sur le serveur
			newFiles = append(newFiles, filePath)
		} else if known && modTime.After(lastMod) {
			// Fichier modifié localement
			modifiedFiles = append(modifiedFiles, filePath)
		} else if !existsOnServer {
			// Fichier connu localement mais absent du serveur
			newFiles = append(newFiles, filePath)
		}
	}

	// Détecter les fichiers supprimés localement
	c.mu.Lock()
	for knownFile := range c.knownFiles {
		if _, exists := allFiles[knownFile]; !exists {
			deletedFiles = append(deletedFiles, knownFile)
		}
	}
	c.mu.Unlock()

	// Afficher le résumé
	totalOps := len(newDirs) + len(newFiles) + len(modifiedFiles) + len(deletedDirs) + len(deletedFiles)
	if totalOps == 0 {
		addLog("✅ Aucune modification à envoyer")
		c.isProcessing = false
		return
	}

	addLog(fmt.Sprintf("📊 Résumé: %d dossiers, %d nouveaux fichiers, %d modifiés, %d supprimés",
		len(newDirs), len(newFiles), len(modifiedFiles), len(deletedDirs)+len(deletedFiles)))

	sent := 0

	// 1. Envoyer les suppressions de dossiers
	for _, dirPath := range deletedDirs {
		change := FileChange{
			FileName: dirPath,
			Op:       "remove",
			IsDir:    true,
			Origin:   "client",
		}
		if err := c.WriteJSONSafe(change); err == nil {
			c.mu.Lock()
			delete(c.knownDirs, dirPath)
			c.mu.Unlock()
			sent++
		}
		time.Sleep(30 * time.Millisecond)
	}

	// 2. Envoyer les suppressions de fichiers
	for _, filePath := range deletedFiles {
		change := FileChange{
			FileName: filePath,
			Op:       "remove",
			IsDir:    false,
			Origin:   "client",
		}
		if err := c.WriteJSONSafe(change); err == nil {
			c.mu.Lock()
			delete(c.knownFiles, filePath)
			c.mu.Unlock()
			sent++
		}
		time.Sleep(30 * time.Millisecond)
	}

	// 3. Envoyer les nouveaux dossiers (triés par profondeur)
	sortedDirs := getSortedKeysByDepth(newDirs)
	for _, dirPath := range sortedDirs {
		change := FileChange{
			FileName: dirPath,
			Op:       "mkdir",
			IsDir:    true,
			Origin:   "client",
		}
		if err := c.WriteJSONSafe(change); err == nil {
			c.mu.Lock()
			c.knownDirs[dirPath] = time.Now()
			c.mu.Unlock()
			sent++
		}
		time.Sleep(30 * time.Millisecond)
	}

	// 4. Envoyer les nouveaux fichiers
	for i, filePath := range newFiles {
		if err := c.sendFile(filePath); err == nil {
			sent++
		}
		// Rate limiting pour éviter la surcharge
		if i > 0 && i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}

	// 5. Envoyer les fichiers modifiés
	for i, filePath := range modifiedFiles {
		if err := c.sendFile(filePath); err == nil {
			sent++
		}
		// Rate limiting
		if i > 0 && i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}

	c.isProcessing = false
	addLog(fmt.Sprintf("✅ %d opérations envoyées avec succès", sent))
	
	// Vider les pending actions après envoi réussi
	GetPendingActions().Clear()
}

// sendFile envoie un fichier au serveur
func (c *Client) sendFile(relPath string) error {
	fullPath := filepath.Join(c.localDir, filepath.FromSlash(relPath))

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	info, _ := os.Stat(fullPath)

	change := FileChange{
		FileName: relPath,
		Op:       "write",
		Content:  base64.StdEncoding.EncodeToString(data),
		IsDir:    false,
		Origin:   "client",
	}

	if err := c.WriteJSONSafe(change); err != nil {
		return err
	}

	c.mu.Lock()
	if info != nil {
		c.knownFiles[relPath] = info.ModTime()
	} else {
		c.knownFiles[relPath] = time.Now()
	}
	c.mu.Unlock()

	return nil
}

// getServerState récupère l'état connu des fichiers (basé sur le cache local)
func (c *Client) getServerState() (map[string]time.Time, map[string]time.Time) {
	serverFiles := make(map[string]time.Time)
	serverDirs := make(map[string]time.Time)

	c.mu.Lock()
	for k, v := range c.knownFiles {
		serverFiles[k] = v
	}
	for k, v := range c.knownDirs {
		serverDirs[k] = v
	}
	c.mu.Unlock()

	return serverFiles, serverDirs
}

// getSortedKeysByDepth trie les chemins par profondeur (moins profond d'abord)
func getSortedKeysByDepth(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}

	// Trier par nombre de séparateurs
	sorted := make([]string, len(paths))
	copy(sorted, paths)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			depthI := strings.Count(sorted[i], "/")
			depthJ := strings.Count(sorted[j], "/")
			if depthI > depthJ {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
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
	c.skipTracking = true // Ignorer le tracking pendant la suppression
	defer func() { 
		c.isProcessing = false 
		c.skipTracking = false
	}()
	
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
	
	// Vider les pending actions car les fichiers n'existent plus
	GetPendingActions().Clear()
	
	time.Sleep(300 * time.Millisecond)
	
	addLog(fmt.Sprintf("✅ %d éléments supprimés", count))
}

func (c *Client) watchRecursive() {
	if c.watcherActive {
		return
	}
	c.watcherActive = true
	defer func() {
		c.watcherActive = false
		close(c.watcherDone)
	}()
	
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
		select {
		case <-c.ctx.Done():
			return
			
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			
			// Tracker le changement local SEULEMENT si sync auto désactivée
			// Si sync auto active, pas besoin de tracker car envoyé immédiatement
			if !c.autoSync {
				c.TrackLocalChange(event)
			}
			
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
			
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			if !c.shouldExit {
				addLog(fmt.Sprintf("⚠️ Erreur watcher: %v", err))
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// TrackLocalChange ajoute un changement local aux pending actions
// Ne s'applique que si la sync auto est désactivée et skipTracking est false
func (c *Client) TrackLocalChange(event fsnotify.Event) {
	// Ignorer si on est en train de recevoir ou vider les fichiers
	if c.skipTracking {
		return
	}
	
	relPath, err := filepath.Rel(c.localDir, event.Name)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)

	// Vérifier les filtres
	filterConfig := GetFilterConfig()
	if filterConfig.Filters.Path.ShouldFilter(relPath) {
		return
	}

	pendingActions := GetPendingActions()

	// Suppression
	if event.Op&fsnotify.Remove != 0 {
		// Vérifier si c'était un dossier ou fichier connu
		c.mu.Lock()
		_, wasDir := c.knownDirs[relPath]
		_, wasFile := c.knownFiles[relPath]
		c.mu.Unlock()

		if wasDir || wasFile {
			pendingActions.Add(&PendingAction{
				Type:  ActionDelete,
				Path:  relPath,
				IsDir: wasDir,
			})
		}
		return
	}

	// Création ou modification
	if event.Op&fsnotify.Create != 0 || event.Op&fsnotify.Write != 0 {
		info, err := os.Stat(event.Name)
		if err != nil {
			return
		}

		if info.IsDir() {
			// Nouveau dossier
			c.mu.Lock()
			_, known := c.knownDirs[relPath]
			c.mu.Unlock()

			if !known {
				pendingActions.Add(&PendingAction{
					Type:    ActionCreate,
					Path:    relPath,
					IsDir:   true,
					ModTime: info.ModTime(),
				})
			}
		} else {
			// Vérifier filtrage par extension et taille
			if filterConfig.ShouldFilterFile(relPath, info.Size(), false) {
				return
			}

			c.mu.Lock()
			_, known := c.knownFiles[relPath]
			c.mu.Unlock()

			actionType := ActionModify
			if !known {
				actionType = ActionCreate
			}

			pendingActions.Add(&PendingAction{
				Type:    actionType,
				Path:    relPath,
				Size:    info.Size(),
				IsDir:   false,
				ModTime: info.ModTime(),
			})
		}
	}
}

func (c *Client) handleLocalEvent(event fsnotify.Event) {
	relPath, err := filepath.Rel(c.localDir, event.Name)
	if err != nil {
		return
	}
	relPath = filepath.ToSlash(relPath)

	// Vérifier le filtrage par chemin/dossier
	filterConfig := GetFilterConfig()
	if filterConfig.Filters.Path.ShouldFilter(relPath) {
		return // Fichier/dossier filtré
	}

	c.mu.Lock()
	if until, exists := c.skipNext[relPath]; exists && time.Now().Before(until) {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	time.Sleep(50 * time.Millisecond)

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
			c.WriteJSONSafe(change)
		} else {
			// Vérifier le filtrage par extension et taille
			if filterConfig.ShouldFilterFile(relPath, info.Size(), false) {
				addLog(fmt.Sprintf("🔍 Fichier ignoré (filtre): %s", relPath))
				return
			}

			time.Sleep(50 * time.Millisecond)
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
			c.WriteJSONSafe(change)
		}
		time.Sleep(50 * time.Millisecond)
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

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.shouldExit || !c.autoSync {
				return
			}

			time.Sleep(100 * time.Millisecond)

			currentFiles := make(map[string]time.Time)
			currentDirs := make(map[string]time.Time)
			c.scanCurrentState(c.localDir, "", currentFiles, currentDirs)

			c.mu.Lock()

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
					c.mu.Unlock()
					c.WriteJSONSafe(change)
					c.mu.Lock()
					delete(c.knownDirs, oldDir)
					dirsRemoved++
					time.Sleep(50 * time.Millisecond)
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
					c.mu.Unlock()
					c.WriteJSONSafe(change)
					c.mu.Lock()
					c.knownDirs[newDir] = modTime
					dirsCreated++
					time.Sleep(50 * time.Millisecond)
				}
			}

			for name, modTime := range currentFiles {
				if until, exists := c.skipNext[name]; exists && time.Now().Before(until) {
					continue
				}

				lastMod, known := c.lastState[name]
				if !known || modTime.After(lastMod) {
					c.mu.Unlock()
					time.Sleep(30 * time.Millisecond)
					c.sendFileNow(name)
					c.mu.Lock()
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
					c.mu.Unlock()
					c.WriteJSONSafe(change)
					c.mu.Lock()
					delete(c.knownFiles, oldFile)
					filesRemoved++
					time.Sleep(50 * time.Millisecond)
				}
			}

			c.lastState = currentFiles
			c.lastDirs = currentDirs
			c.mu.Unlock()

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

	time.Sleep(30 * time.Millisecond)

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

	c.WriteJSONSafe(change)
	c.mu.Lock()
	c.knownFiles[relPath] = time.Now()
	c.mu.Unlock()
	time.Sleep(50 * time.Millisecond)
}

// ScanAndDetectDifferences scanne le dossier local et détecte les fichiers
// qui n'existent pas sur le serveur pour les ajouter aux pending actions
func (c *Client) ScanAndDetectDifferences() {
	filterConfig := GetFilterConfig()
	pendingActions := GetPendingActions()
	
	// Scanner tous les fichiers et dossiers locaux
	localFiles := make(map[string]os.FileInfo)
	localDirs := make(map[string]os.FileInfo)
	
	filepath.Walk(c.localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		relPath, _ := filepath.Rel(c.localDir, path)
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		
		if info.IsDir() {
			if !filterConfig.Filters.Path.ShouldFilter(relPath) {
				localDirs[relPath] = info
			}
		} else {
			if !filterConfig.ShouldFilterFile(relPath, info.Size(), false) {
				localFiles[relPath] = info
			}
		}
		return nil
	})
	
	// Obtenir l'état connu du serveur
	serverFiles, serverDirs := c.getServerState()
	
	detectedCount := 0
	
	// Détecter les nouveaux dossiers
	for dirPath, info := range localDirs {
		if _, existsOnServer := serverDirs[dirPath]; !existsOnServer {
			pendingActions.Add(&PendingAction{
				Type:    ActionCreate,
				Path:    dirPath,
				IsDir:   true,
				ModTime: info.ModTime(),
			})
			detectedCount++
		}
	}
	
	// Détecter les nouveaux fichiers ou modifiés
	for filePath, info := range localFiles {
		serverModTime, existsOnServer := serverFiles[filePath]
		
		if !existsOnServer {
			// Nouveau fichier
			pendingActions.Add(&PendingAction{
				Type:    ActionCreate,
				Path:    filePath,
				Size:    info.Size(),
				IsDir:   false,
				ModTime: info.ModTime(),
			})
			detectedCount++
		} else if info.ModTime().After(serverModTime) {
			// Fichier modifié localement (plus récent que le serveur)
			pendingActions.Add(&PendingAction{
				Type:    ActionModify,
				Path:    filePath,
				Size:    info.Size(),
				IsDir:   false,
				ModTime: info.ModTime(),
			})
			detectedCount++
		}
	}
	
	if detectedCount > 0 {
		addLog(fmt.Sprintf("📋 %d différences locales détectées", detectedCount))
	}
}

// DownloadBackup télécharge tous les fichiers vers un dossier Backup
// Approche: D'abord synchroniser depuis le serveur, puis copier le dossier local
func (c *Client) DownloadBackup(destDir string) {
	if c.isProcessing {
		addLog("⏳ Operation deja en cours...")
		return
	}
	
	// Créer le dossier de backup avec timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupDir := filepath.Join(destDir, fmt.Sprintf("Backup_Spiralydata_%s", timestamp))
	
	addLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	addLog("📦 DEBUT DE LA BACKUP")
	addLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Lancer en goroutine pour ne pas bloquer l'UI
	go c.performBackupViaPull(backupDir)
}

func (c *Client) performBackupViaPull(backupDir string) {
	c.isProcessing = true
	c.skipTracking = true
	defer func() {
		c.isProcessing = false
		c.skipTracking = false
	}()
	
	// === ETAPE 1: Compter les fichiers sur le serveur ===
	addLog("📋 Etape 1: Scan du serveur...")
	
	c.explorerActive = true
	c.treeItemsChan = make(chan FileTreeItemMessage, 1000)
	
	if err := c.WriteJSONSafe(map[string]string{"type": "request_file_tree", "origin": "client"}); err != nil {
		addLog(fmt.Sprintf("❌ Erreur: %v", err))
		c.explorerActive = false
		return
	}
	
	// Compter les éléments
	totalElements := 0
	timeout1 := time.After(60 * time.Second)
	
countLoop:
	for {
		select {
		case <-timeout1:
			break countLoop
		case item, ok := <-c.treeItemsChan:
			if !ok {
				break countLoop
			}
			if item.Type == "file_tree_complete" {
				break countLoop
			}
			if item.Type == "file_tree_item" {
				totalElements++
			}
		}
	}
	c.explorerActive = false
	c.treeItemsChan = nil
	
	if totalElements == 0 {
		addLog("⚠️ Aucun fichier sur le serveur")
		return
	}
	
	addLog(fmt.Sprintf("✅ %d elements trouves sur le serveur", totalElements))
	
	// === ETAPE 2: Recevoir tous les fichiers ===
	addLog("📥 Etape 2: Reception des fichiers...")
	
	reqMsg := map[string]string{
		"type":   "request_all_files",
		"origin": "client",
	}
	
	if err := c.WriteJSONSafe(reqMsg); err != nil {
		addLog(fmt.Sprintf("❌ Erreur: %v", err))
		return
	}
	
	// Attendre la réception en comptant les fichiers locaux
	lastCount := countLocalFiles(c.localDir)
	stableCount := 0
	maxWait := 60 // secondes max
	
	for i := 0; i < maxWait; i++ {
		time.Sleep(1 * time.Second)
		
		currentCount := countLocalFiles(c.localDir)
		
		// Log de progression tous les 2 secondes
		if i%2 == 0 {
			addLog(fmt.Sprintf("⏳ Fichiers locaux: %d / %d attendus", currentCount, totalElements))
		}
		
		// Vérifier la stabilité (pas de nouveaux fichiers depuis 3 secondes)
		if currentCount == lastCount {
			stableCount++
			if stableCount >= 3 {
				addLog(fmt.Sprintf("✅ Reception terminee: %d fichiers", currentCount))
				break
			}
		} else {
			stableCount = 0
		}
		
		lastCount = currentCount
	}
	
	// Délai pour s'assurer que les écritures sont finies
	time.Sleep(1 * time.Second)
	
	// === ETAPE 3: Copier le dossier local vers la backup ===
	addLog("📁 Etape 3: Copie vers le dossier de backup...")
	
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		addLog(fmt.Sprintf("❌ Erreur creation dossier: %v", err))
		return
	}
	
	filesCopied := 0
	dirsCopied := 0
	errors := 0
	
	err := filepath.Walk(c.localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		relPath, err := filepath.Rel(c.localDir, path)
		if err != nil || relPath == "." {
			return nil
		}
		
		destPath := filepath.Join(backupDir, relPath)
		
		if info.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err == nil {
				dirsCopied++
			}
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				errors++
				return nil
			}
			
			os.MkdirAll(filepath.Dir(destPath), 0755)
			
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				errors++
				return nil
			}
			filesCopied++
		}
		
		return nil
	})
	
	if err != nil {
		addLog(fmt.Sprintf("⚠️ Erreur parcours: %v", err))
	}
	
	addLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	addLog("✅ BACKUP TERMINEE")
	addLog(fmt.Sprintf("📁 Dossiers: %d", dirsCopied))
	addLog(fmt.Sprintf("📄 Fichiers: %d", filesCopied))
	if errors > 0 {
		addLog(fmt.Sprintf("⚠️ Erreurs: %d", errors))
	}
	addLog(fmt.Sprintf("📍 %s", backupDir))
	addLog("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// countLocalFiles compte le nombre de fichiers (pas dossiers) dans un répertoire
func countLocalFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
} 