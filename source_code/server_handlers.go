package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

// ============================================================
// FONCTIONS SERVEUR
// ============================================================

func (s *Server) sendAllFilesAndDirs(ws *websocket.Conn) {
	addLog("📤 Début envoi structure...")
	time.Sleep(200 * time.Millisecond)
	s.sendDirRecursiveWithDelay(ws, s.WatchDir, "", 0)
	time.Sleep(300 * time.Millisecond)
	addLog("✅ Envoi structure terminé")
}

func (s *Server) sendDirRecursiveWithDelay(ws *websocket.Conn, basePath, relPath string, level int) {
	fullPath := filepath.Join(basePath, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return
	}

	var dirs []os.DirEntry
	var files []os.DirEntry
	
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}
	
	for i, entry := range dirs {
		itemRelPath := filepath.ToSlash(filepath.Join(relPath, entry.Name()))
		
		ws.WriteJSON(FileChange{
			FileName: itemRelPath,
			Op:       "mkdir",
			IsDir:    true,
			Origin:   "server",
		})
		
		baseDelay := 80 * time.Millisecond
		if level > 2 {
			baseDelay = 50 * time.Millisecond
		}
		time.Sleep(baseDelay)
		
		if i > 0 && i%5 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
		
		s.sendDirRecursiveWithDelay(ws, basePath, filepath.Join(relPath, entry.Name()), level+1)
	}
	
	for i, entry := range files {
		itemRelPath := filepath.ToSlash(filepath.Join(relPath, entry.Name()))
		fullFilePath := filepath.Join(basePath, relPath, entry.Name())
		
		data, err := readFileWithRetry(fullFilePath)
		if err != nil {
			continue
		}
		
		ws.WriteJSON(FileChange{
			FileName: itemRelPath,
			Op:       "create",
			Content:  base64.StdEncoding.EncodeToString(data),
			IsDir:    false,
			Origin:   "server",
		})
		
		time.Sleep(60 * time.Millisecond)
		
		if i > 0 && i%10 == 0 {
			time.Sleep(150 * time.Millisecond)
		}
	}
}

func (s *Server) watchRecursive() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		addLog("❌ Erreur watcher: " + err.Error())
		return
	}
	defer watcher.Close()

	time.Sleep(200 * time.Millisecond)
	s.addDirToWatcher(watcher, s.WatchDir)

	for {
		if s.shouldExit {
			return
		}
		select {
		case event := <-watcher.Events:
			time.Sleep(50 * time.Millisecond)
			s.handleEvent(event)
			
			if event.Op&fsnotify.Create != 0 {
				time.Sleep(100 * time.Millisecond)
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}
			
		case err := <-watcher.Errors:
			if !s.shouldExit {
				addLog("⚠️ Erreur watcher: " + err.Error())
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Server) addDirToWatcher(watcher *fsnotify.Watcher, dir string) {
	watcher.Add(dir)
	time.Sleep(20 * time.Millisecond)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	
	for i, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			s.addDirToWatcher(watcher, subDir)
			
			if i > 0 && i%5 == 0 {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

func (s *Server) handleEvent(event fsnotify.Event) {
	relPath, err := filepath.Rel(s.WatchDir, event.Name)
	if err != nil || relPath == "." {
		return
	}

	relPath = filepath.ToSlash(filepath.Clean(relPath))
	
	s.mu.Lock()
	
	if until, exists := s.skipNext[relPath]; exists && time.Now().Before(until) {
		s.mu.Unlock()
		return
	}
	
	if until, exists := s.pendingMoves[relPath]; exists && time.Now().Before(until) {
		s.mu.Unlock()
		return
	}
	
	s.mu.Unlock()

	info, err := os.Stat(event.Name)
	isDir := err == nil && info.IsDir()

	if event.Op&fsnotify.Create != 0 {
		s.mu.Lock()
		s.pendingMoves[relPath] = time.Now().Add(500 * time.Millisecond)
		s.mu.Unlock()
		
		time.Sleep(250 * time.Millisecond)
		
		if _, err := os.Stat(event.Name); err != nil {
			return
		}
		
		if isDir {
			s.mu.Lock()
			if _, known := s.knownDirs[relPath]; known {
				s.mu.Unlock()
				return
			}
			s.knownDirs[relPath] = time.Now()
			s.mu.Unlock()
			
			msg := FileChange{
				FileName: relPath,
				Op:       "mkdir",
				IsDir:    true,
				Origin:   "server",
			}
			s.broadcast(msg)
			addLog("📤 Dossier créé: " + relPath)
			time.Sleep(150 * time.Millisecond)
		} else {
			s.mu.Lock()
			if _, known := s.knownFiles[relPath]; known {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			
			data, err := readFileWithRetry(event.Name)
			if err != nil {
				return
			}
			
			s.mu.Lock()
			s.knownFiles[relPath] = time.Now()
			s.mu.Unlock()
			
			msg := FileChange{
				FileName: relPath,
				Op:       "create",
				Content:  base64.StdEncoding.EncodeToString(data),
				IsDir:    false,
				Origin:   "server",
			}
			s.broadcast(msg)
			addLog("📤 Nouveau: " + relPath)
			time.Sleep(150 * time.Millisecond)
		}
	}

	if event.Op&fsnotify.Write != 0 && !isDir {
		time.Sleep(100 * time.Millisecond)
		
		data, err := readFileWithRetry(event.Name)
		if err != nil {
			return
		}
		
		s.mu.Lock()
		s.knownFiles[relPath] = time.Now()
		s.mu.Unlock()
		
		msg := FileChange{
			FileName: relPath,
			Op:       "write",
			Content:  base64.StdEncoding.EncodeToString(data),
			IsDir:    false,
			Origin:   "server",
		}
		s.broadcast(msg)
		addLog("📤 Modifié: " + relPath)
		time.Sleep(150 * time.Millisecond)
	}

	if event.Op&fsnotify.Remove != 0 {
		s.mu.Lock()
		wasDir := s.knownDirs[relPath] != time.Time{}
		
		if wasDir {
			delete(s.knownDirs, relPath)
		} else {
			delete(s.knownFiles, relPath)
		}
		s.mu.Unlock()
		
		msg := FileChange{
			FileName: relPath,
			Op:       "remove",
			IsDir:    wasDir,
			Origin:   "server",
		}
		s.broadcast(msg)
		
		if wasDir {
			addLog("📤 Dossier supprimé: " + relPath)
		} else {
			addLog("📤 Supprimé: " + relPath)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (s *Server) applyChange(msg FileChange) {
	normalizedPath := filepath.FromSlash(msg.FileName)
	path := filepath.Join(s.WatchDir, normalizedPath)

	s.mu.Lock()
	s.skipNext[msg.FileName] = time.Now().Add(5 * time.Second)
	s.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	switch msg.Op {
	case "mkdir":
		os.MkdirAll(path, 0755)
		s.mu.Lock()
		s.knownDirs[msg.FileName] = time.Now()
		s.mu.Unlock()
		
	case "create", "write":
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)
		
		data, err := base64.StdEncoding.DecodeString(msg.Content)
		if err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(path, data, 0644)
		s.mu.Lock()
		s.knownFiles[msg.FileName] = time.Now()
		s.mu.Unlock()
		
	case "remove":
		if msg.IsDir {
			os.RemoveAll(path)
			s.mu.Lock()
			delete(s.knownDirs, msg.FileName)
			s.mu.Unlock()
		} else {
			os.Remove(path)
			s.mu.Lock()
			delete(s.knownFiles, msg.FileName)
			s.mu.Unlock()
		}
	}
	
	time.Sleep(100 * time.Millisecond)
}

func (s *Server) periodicCheck() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if s.shouldExit {
			return
		}

		time.Sleep(200 * time.Millisecond)

		currentFiles := make(map[string]time.Time)
		currentDirs := make(map[string]time.Time)
		s.scanCurrentState(s.WatchDir, "", currentFiles, currentDirs)

		s.mu.Lock()
		
		for oldDir := range s.knownDirs {
			if until, exists := s.skipNext[oldDir]; exists && time.Now().Before(until) {
				continue
			}
			if _, exists := currentDirs[oldDir]; !exists {
				msg := FileChange{
					FileName: oldDir,
					Op:       "remove",
					IsDir:    true,
					Origin:   "server",
				}
				delete(s.knownDirs, oldDir)
				for c := range s.Clients {
					c.WriteJSON(msg)
				}
				addLog("🗑️ Dossier supprimé: " + oldDir)
				time.Sleep(150 * time.Millisecond)
			}
		}
		
		for oldFile := range s.knownFiles {
			if until, exists := s.skipNext[oldFile]; exists && time.Now().Before(until) {
				continue
			}
			if _, exists := currentFiles[oldFile]; !exists {
				msg := FileChange{
					FileName: oldFile,
					Op:       "remove",
					IsDir:    false,
					Origin:   "server",
				}
				delete(s.knownFiles, oldFile)
				for c := range s.Clients {
					c.WriteJSON(msg)
				}
				addLog("🗑️ Supprimé: " + oldFile)
				time.Sleep(150 * time.Millisecond)
			}
		}

		for newDir, modTime := range currentDirs {
			if _, exists := s.knownDirs[newDir]; !exists {
				if until, exists := s.skipNext[newDir]; exists && time.Now().Before(until) {
					continue
				}
				msg := FileChange{
					FileName: newDir,
					Op:       "mkdir",
					IsDir:    true,
					Origin:   "server",
				}
				s.knownDirs[newDir] = modTime
				for c := range s.Clients {
					c.WriteJSON(msg)
				}
				addLog("📤 Dossier créé: " + newDir)
				time.Sleep(150 * time.Millisecond)
			}
		}

		for name, modTime := range currentFiles {
			if t, exists := s.knownFiles[name]; !exists || modTime.After(t) {
				if until, exists := s.skipNext[name]; exists && time.Now().Before(until) {
					continue
				}

				data, err := readFileWithRetry(filepath.Join(s.WatchDir, name))
				if err == nil {
					msg := FileChange{
						FileName: name,
						Op:       "write",
						Content:  base64.StdEncoding.EncodeToString(data),
						IsDir:    false,
						Origin:   "server",
					}
					for c := range s.Clients {
						c.WriteJSON(msg)
					}
					s.knownFiles[name] = modTime
					addLog("📤 Modifié: " + name)
					time.Sleep(150 * time.Millisecond)
				}
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) scanCurrentState(basePath, relPath string, files map[string]time.Time, dirs map[string]time.Time) {
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
			s.scanCurrentState(basePath, filepath.Join(relPath, entry.Name()), files, dirs)
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