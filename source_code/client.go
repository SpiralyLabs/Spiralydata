package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/gorilla/websocket"
)

type Client struct {
	ws                 *websocket.Conn
	wsMu               sync.Mutex // Mutex pour les écritures WebSocket
	localDir           string
	mu                 sync.Mutex
	skipNext           map[string]time.Time
	knownFiles         map[string]time.Time
	knownDirs          map[string]time.Time
	lastState          map[string]time.Time
	lastDirs           map[string]time.Time
	scanRunning        bool
	shouldExit         bool
	autoSync           bool
	localChanges       []FileChange
	connectionTime     time.Time
	isProcessing       bool
	watcherActive      bool
	pendingChanges     []FileChange
	pendingMu          sync.Mutex
	filesReceivedCount int
	lastLogTime        time.Time
	explorerActive     bool
	treeItemsChan      chan FileTreeItemMessage
	downloadActive     bool
	downloadChan       chan FileChange
	ctx                context.Context
	cancel             context.CancelFunc
	watcherDone        chan struct{}
	opQueue            chan func() // Queue d'opérations pour éviter les race conditions
	skipTracking       bool        // Ignorer le tracking pendant Recevoir/Vider local
}

func StartClientGUI(serverAddr, hostID, syncDir string, stopAnimation, connectionSuccess *bool, loadingLabel, statusLabel, infoLabel *widget.Label, client **Client) {
	addLog("🔌 Connexion au serveur " + serverAddr)
	
	time.Sleep(300 * time.Millisecond)
	
	dialer := &websocket.Dialer{
		HandshakeTimeout:  10 * time.Second,
		ReadBufferSize:    10 * 1024 * 1024, // 10MB
		WriteBufferSize:   10 * 1024 * 1024, // 10MB
	}
	
	ws, _, err := dialer.Dial("ws://"+serverAddr+"/ws", nil)
	if err != nil {
		addLog(fmt.Sprintf("❌ Impossible de se connecter: %v", err))
		*stopAnimation = true
		loadingLabel.SetText("✗ Connexion échouée")
		loadingLabel.Refresh()
		statusLabel.SetText("Statut: Échec de connexion")
		statusLabel.Refresh()
		infoLabel.SetText(fmt.Sprintf(
			"ÉCHEC DE CONNEXION\n\n"+
				"Serveur: %s\n"+
				"ID: %s\n"+
				"Dossier: %s\n\n"+
				"Impossible de se connecter au serveur.\n"+
				"Vérifiez l'adresse IP et le port.",
			serverAddr, hostID, syncDir,
		))
		infoLabel.Refresh()
		return
	}
	
	// Augmenter la limite de lecture pour les gros fichiers
	ws.SetReadLimit(50 * 1024 * 1024) // 50MB
	
	addLog("✅ Connexion WebSocket établie")

	time.Sleep(200 * time.Millisecond)

	authReq := AuthRequest{
		Type:   "auth_request",
		HostID: hostID,
	}

	addLog("🔐 Authentification en cours...")
	if err := ws.WriteJSON(authReq); err != nil {
		addLog(fmt.Sprintf("❌ Erreur d'authentification: %v", err))
		ws.Close()
		*stopAnimation = true
		loadingLabel.SetText("✗ Erreur d'authentification")
		loadingLabel.Refresh()
		statusLabel.SetText("Statut: Erreur d'authentification")
		statusLabel.Refresh()
		return
	}

	time.Sleep(300 * time.Millisecond)

	addLog("⏳ Attente de la réponse...")
	ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	var authResp AuthResponse
	if err := ws.ReadJSON(&authResp); err != nil {
		addLog(fmt.Sprintf("❌ Pas de réponse du serveur: %v", err))
		ws.Close()
		*stopAnimation = true
		loadingLabel.SetText("✗ Pas de réponse")
		loadingLabel.Refresh()
		statusLabel.SetText("Statut: Pas de réponse")
		statusLabel.Refresh()
		return
	}
	ws.SetReadDeadline(time.Time{})

	if authResp.Type == "auth_failed" {
		addLog(fmt.Sprintf("🚫 Authentification refusée: %s", authResp.Message))
		ws.Close()
		*stopAnimation = true
		loadingLabel.SetText("✗ ID incorrect")
		loadingLabel.Refresh()
		statusLabel.SetText("Statut: ID incorrect")
		statusLabel.Refresh()
		infoLabel.SetText(fmt.Sprintf(
			"AUTHENTIFICATION REFUSÉE\n\n"+
				"Serveur: %s\n"+
				"ID: %s\n"+
				"Dossier: %s\n\n"+
				"L'ID du host est incorrect.\n"+
				"Vérifiez l'ID et réessayez.",
			serverAddr, hostID, syncDir,
		))
		infoLabel.Refresh()
		return
	}

	*stopAnimation = true
	*connectionSuccess = true
	addLog(fmt.Sprintf("🎉 Connecté au serveur %s", serverAddr))
	addLog(fmt.Sprintf("🔒 ID validé: %s", hostID))
	
	time.Sleep(200 * time.Millisecond)
	
	loadingLabel.SetText("✓ Connecté")
	loadingLabel.Refresh()
	statusLabel.SetText("Statut: Connecté (Mode Manuel)")
	statusLabel.Refresh()
	
	infoLabel.SetText(fmt.Sprintf(
		"CONNECTÉ\n\n"+
			"Serveur: %s\n"+
			"ID: %s\n"+
			"Dossier: %s\n\n"+
			"Mode: Manuel\n"+
			"Activez la sync pour synchroniser automatiquement",
		serverAddr, hostID, syncDir,
	))
	infoLabel.Refresh()

	if err := os.MkdirAll(syncDir, 0755); err != nil {
		addLog(fmt.Sprintf("❌ Impossible de créer le dossier: %v", err))
	} else {
		addLog(fmt.Sprintf("📂 Dossier: %s", syncDir))
	}

	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	*client = &Client{
		ws:                 ws,
		localDir:           syncDir,
		skipNext:           make(map[string]time.Time),
		knownFiles:         make(map[string]time.Time),
		knownDirs:          make(map[string]time.Time),
		lastState:          make(map[string]time.Time),
		lastDirs:           make(map[string]time.Time),
		shouldExit:         false,
		autoSync:           false,
		localChanges:       []FileChange{},
		connectionTime:     time.Now(),
		isProcessing:       false,
		watcherActive:      false,
		pendingChanges:     []FileChange{},
		filesReceivedCount: 0,
		lastLogTime:        time.Now(),
		explorerActive:     false,
		downloadActive:     false,
		ctx:                ctx,
		cancel:             cancel,
		watcherDone:        make(chan struct{}),
		opQueue:            make(chan func(), 100),
	}

	// Démarrer le worker pour traiter les opérations
	go (*client).processOperationQueue()

	addLog("🔍 Scan initial du dossier local...")
	time.Sleep(200 * time.Millisecond)
	(*client).scanInitial()
	
	// Détecter les différences avec le serveur pour les ajouter aux pending actions
	(*client).ScanAndDetectDifferences()
	
	addLog("✅ Client prêt - Mode Manuel")
	addLog("👀 En attente de commandes...")

	time.Sleep(300 * time.Millisecond)
	go (*client).watchRecursive()

	for {
		var rawMsg json.RawMessage
		if err := ws.ReadJSON(&rawMsg); err != nil {
			if !(*client).shouldExit {
				addLog("💔 Connexion perdue")
				*connectionSuccess = false
				loadingLabel.SetText("✗ Connexion perdue")
				loadingLabel.Refresh()
				statusLabel.SetText("Statut: Déconnecté")
				statusLabel.Refresh()
			}
			
			(*client).cleanup()
			ws.Close()
			break
		}

		time.Sleep(30 * time.Millisecond)

		var treeItem FileTreeItemMessage
		if err := json.Unmarshal(rawMsg, &treeItem); err == nil {
			if treeItem.Type == "file_tree_item" || treeItem.Type == "file_tree_complete" {
				// Toujours essayer d'envoyer si le channel existe
				if (*client).treeItemsChan != nil {
					select {
					case (*client).treeItemsChan <- treeItem:
						// Message envoyé
					default:
						// Channel plein ou fermé, ignorer
					}
				}
				continue
			}
		}

		var msg FileChange
		if err := json.Unmarshal(rawMsg, &msg); err == nil {
			if msg.Origin != "client" {
				if (*client).downloadActive {
					(*client).downloadChan <- msg
					continue
				}
				
				if (*client).autoSync {
					(*client).filesReceivedCount++
					if time.Since((*client).lastLogTime) > 2*time.Second {
						if (*client).filesReceivedCount > 0 {
							addLog(fmt.Sprintf("📥 %d fichiers reçus", (*client).filesReceivedCount))
							(*client).filesReceivedCount = 0
							(*client).lastLogTime = time.Now()
						}
					}
					
					time.Sleep(50 * time.Millisecond)
					(*client).applyChange(msg)
				} else {
					if (*client).isProcessing {
						(*client).filesReceivedCount++
						if time.Since((*client).lastLogTime) > 2*time.Second {
							if (*client).filesReceivedCount > 0 {
								addLog(fmt.Sprintf("📥 Réception: %d fichiers", (*client).filesReceivedCount))
								(*client).filesReceivedCount = 0
								(*client).lastLogTime = time.Now()
							}
						}
						
						time.Sleep(50 * time.Millisecond)
						(*client).applyChange(msg)
					} else {
						(*client).pendingMu.Lock()
						(*client).pendingChanges = append((*client).pendingChanges, msg)
						(*client).pendingMu.Unlock()
					}
				}
			}
		}
	}
}

func (c *Client) cleanup() {
	c.mu.Lock()
	if c.shouldExit {
		c.mu.Unlock()
		return // Déjà en cours de cleanup
	}
	c.shouldExit = true
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.watcherActive {
		c.watcherActive = false
		select {
		case <-c.watcherDone:
		case <-time.After(2 * time.Second):
		}
	}

	if c.explorerActive {
		c.explorerActive = false
		if c.treeItemsChan != nil {
			select {
			case <-c.treeItemsChan:
			default:
				close(c.treeItemsChan)
			}
			c.treeItemsChan = nil
		}
	}

	if c.downloadActive {
		c.downloadActive = false
		if c.downloadChan != nil {
			select {
			case <-c.downloadChan:
			default:
				close(c.downloadChan)
			}
			c.downloadChan = nil
		}
	}

	// Fermer la queue d'opérations
	if c.opQueue != nil {
		select {
		case <-c.opQueue:
		default:
			close(c.opQueue)
		}
		c.opQueue = nil
	}
}

// processOperationQueue traite les opérations en arrière-plan
func (c *Client) processOperationQueue() {
	for op := range c.opQueue {
		if c.shouldExit {
			return
		}
		op()
	}
}

// WriteJSONSafe envoie un message JSON de manière thread-safe
func (c *Client) WriteJSONSafe(v interface{}) error {
	c.wsMu.Lock()
	defer c.wsMu.Unlock()

	if c.ws == nil {
		return fmt.Errorf("connexion WebSocket fermée")
	}

	// Définir un timeout pour l'écriture
	c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := c.ws.WriteJSON(v)
	c.ws.SetWriteDeadline(time.Time{}) // Reset le deadline

	return err
}

func (c *Client) ToggleAutoSync() {
	c.mu.Lock()
	c.autoSync = !c.autoSync
	status := c.autoSync
	c.mu.Unlock()
	
	time.Sleep(200 * time.Millisecond)
	
	if status {
		addLog("🟢 Synchronisation automatique ACTIVÉE")
		
		// Vider les pending actions car tout sera synchronisé automatiquement
		GetPendingActions().Clear()
		
		time.Sleep(300 * time.Millisecond)
		
		c.pendingMu.Lock()
		pendingCount := len(c.pendingChanges)
		if pendingCount > 0 {
			addLog(fmt.Sprintf("📦 Application de %d changements en attente...", pendingCount))
			for i, change := range c.pendingChanges {
				c.applyChange(change)
				if i > 0 && i%10 == 0 {
					time.Sleep(200 * time.Millisecond)
				}
			}
			c.pendingChanges = []FileChange{}
			addLog(fmt.Sprintf("✅ %d changements appliqués", pendingCount))
		}
		c.pendingMu.Unlock()
		
		time.Sleep(300 * time.Millisecond)
		go c.periodicScanner()
	} else {
		addLog("🔴 Synchronisation automatique DÉSACTIVÉE")
		
		c.pendingMu.Lock()
		pendingCount := len(c.pendingChanges)
		c.pendingMu.Unlock()
		
		if pendingCount > 0 {
			addLog(fmt.Sprintf("📋 %d changements en attente", pendingCount))
		}
	}
}

func (c *Client) scanInitial() {
	time.Sleep(200 * time.Millisecond)
	c.scanDirRecursive(c.localDir, "")
}

func (c *Client) scanDirRecursive(basePath, relPath string) {
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
				c.lastDirs[itemRelPath] = info.ModTime()
				c.knownDirs[itemRelPath] = info.ModTime()
			}
			c.scanDirRecursive(basePath, filepath.Join(relPath, entry.Name()))
		} else {
			if info != nil {
				c.lastState[itemRelPath] = info.ModTime()
				c.knownFiles[itemRelPath] = info.ModTime()
			}
		}
		
		if i > 0 && i%20 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func (c *Client) applyChange(msg FileChange) {
	normalizedPath := filepath.FromSlash(msg.FileName)
	target := filepath.Join(c.localDir, normalizedPath)

	c.mu.Lock()
	c.skipNext[msg.FileName] = time.Now().Add(5 * time.Second)
	c.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	switch msg.Op {
	case "mkdir":
		os.MkdirAll(target, 0755)
		c.mu.Lock()
		c.knownDirs[msg.FileName] = time.Now()
		c.lastDirs[msg.FileName] = time.Now()
		c.mu.Unlock()
		
	case "remove":
		if msg.IsDir {
			os.RemoveAll(target)
			c.mu.Lock()
			delete(c.knownDirs, msg.FileName)
			delete(c.lastDirs, msg.FileName)
			c.mu.Unlock()
		} else {
			os.Remove(target)
			c.mu.Lock()
			delete(c.knownFiles, msg.FileName)
			delete(c.lastState, msg.FileName)
			c.mu.Unlock()
		}
		
	case "create", "write":
		dir := filepath.Dir(target)
		os.MkdirAll(dir, 0755)
		
		data, _ := base64.StdEncoding.DecodeString(msg.Content)
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(target, data, 0644)
		info, _ := os.Stat(target)
		c.mu.Lock()
		if info != nil {
			c.knownFiles[msg.FileName] = info.ModTime()
			c.lastState[msg.FileName] = info.ModTime()
		}
		c.mu.Unlock()
	}
	
	time.Sleep(100 * time.Millisecond)
}

func getSortedKeys(m map[string]time.Time) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
} 