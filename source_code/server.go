package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	HostID       string
	Clients      map[*websocket.Conn]string
	Upgrader     websocket.Upgrader
	WatchDir     string
	mu           sync.Mutex
	skipNext     map[string]time.Time
	knownFiles   map[string]time.Time
	knownDirs    map[string]time.Time
	clientNum    int
	shouldExit   bool
	httpServer   *http.Server
	pendingMoves map[string]time.Time
}

func NewServer(hostID string) *Server {
	return &Server{
		HostID:  hostID,
		Clients: make(map[*websocket.Conn]string),
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		skipNext:     make(map[string]time.Time),
		knownFiles:   make(map[string]time.Time),
		knownDirs:    make(map[string]time.Time),
		pendingMoves: make(map[string]time.Time),
		clientNum:    0,
		shouldExit:   false,
	}
}

func (s *Server) Start(port string) {
	s.WatchDir = filepath.Join(getExecutableDir(), "Spiralydata")
	os.MkdirAll(s.WatchDir, 0755)

	addLog("🚀 Serveur démarré")
	addLog(fmt.Sprintf("🔑 ID: %s", s.HostID))
	addLog(fmt.Sprintf("📁 Dossier: %s", s.WatchDir))
	addLog("👂 En attente de connexions...")

	s.updateKnownFilesAndDirs()
	go s.watchRecursive()
	go s.periodicCheck()
	go s.cleanPendingMoves()

	http.HandleFunc("/ws", s.handleWS)
	s.httpServer = &http.Server{Addr: ":" + port}
	addLog(fmt.Sprintf("🌐 Port: %s", port))
	
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		addLog(fmt.Sprintf("❌ Erreur serveur: %v", err))
	}
}

func (s *Server) Stop() {
	addLog("🛑 Arrêt du serveur...")
	s.shouldExit = true
	
	s.mu.Lock()
	for client := range s.Clients {
		client.Close()
	}
	s.mu.Unlock()
	
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	
	addLog("✅ Serveur arrêté")
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	addLog(fmt.Sprintf("🔌 Connexion depuis %s", r.RemoteAddr))
	
	ws, err := s.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		addLog(fmt.Sprintf("❌ Erreur WebSocket: %v", err))
		return
	}

	var rawMsg json.RawMessage
	if err := ws.ReadJSON(&rawMsg); err != nil {
		addLog(fmt.Sprintf("❌ Erreur lecture: %v", err))
		ws.Close()
		return
	}

	var authReq AuthRequest
	if err := json.Unmarshal(rawMsg, &authReq); err != nil {
		addLog(fmt.Sprintf("❌ Erreur parsing: %v", err))
		ws.Close()
		return
	}

	if authReq.Type == "auth_request" {
		if authReq.HostID == s.HostID {
			s.mu.Lock()
			s.clientNum++
			clientName := fmt.Sprintf("Client_%d", s.clientNum)
			s.Clients[ws] = clientName
			totalClients := len(s.Clients)
			s.mu.Unlock()

			addLog(fmt.Sprintf("✅ %s connecté", clientName))
			addLog(fmt.Sprintf("👥 Clients: %d", totalClients))

			ws.WriteJSON(AuthResponse{
				Type:    "auth_success",
				Message: "Connexion établie",
			})

			addLog(fmt.Sprintf("📤 Envoi structure à %s...", clientName))
			s.sendAllFilesAndDirs(ws)
			addLog(fmt.Sprintf("✅ Structure envoyée à %s", clientName))
			
			s.handleClientMessages(ws, clientName)

		} else {
			addLog(fmt.Sprintf("🚫 Connexion refusée (ID: %s)", authReq.HostID))
			ws.WriteJSON(AuthResponse{
				Type:    "auth_failed",
				Message: "Identifiant incorrect",
			})
			ws.Close()
			return
		}
	}
}

func (s *Server) handleClientMessages(ws *websocket.Conn, clientName string) {
	defer func() {
		s.mu.Lock()
		delete(s.Clients, ws)
		remaining := len(s.Clients)
		s.mu.Unlock()
		ws.Close()
		addLog(fmt.Sprintf("❌ %s déconnecté", clientName))
		addLog(fmt.Sprintf("👥 Clients restants: %d", remaining))
	}()

	for {
		var rawMsg json.RawMessage
		if err := ws.ReadJSON(&rawMsg); err != nil {
			break
		}
		
		var reqMap map[string]string
		if err := json.Unmarshal(rawMsg, &reqMap); err == nil {
			if reqType, ok := reqMap["type"]; ok {
				if reqType == "request_all_files" {
					addLog(fmt.Sprintf("📥 %s: Demande structure complète", clientName))
					s.sendAllFilesAndDirs(ws)
					addLog(fmt.Sprintf("📤 Structure envoyée à %s", clientName))
					continue
				}
			}
		}
		
		var msg FileChange
		if err := json.Unmarshal(rawMsg, &msg); err == nil {
			if msg.Origin != "server" {
				if msg.IsDir {
					if msg.Op == "mkdir" {
						addLog(fmt.Sprintf("📥 %s: Dossier créé → %s", clientName, msg.FileName))
					} else if msg.Op == "remove" {
						addLog(fmt.Sprintf("📥 %s: Dossier supprimé → %s", clientName, msg.FileName))
					}
				} else {
					if msg.Op == "create" {
						addLog(fmt.Sprintf("📥 %s: Nouveau → %s", clientName, msg.FileName))
					} else if msg.Op == "write" {
						addLog(fmt.Sprintf("📥 %s: Modifié → %s", clientName, msg.FileName))
					} else if msg.Op == "remove" {
						addLog(fmt.Sprintf("📥 %s: Supprimé → %s", clientName, msg.FileName))
					}
				}
				
				s.applyChange(msg)
				s.broadcastExcept(msg, ws)
			}
		}
	}
}

func (s *Server) broadcast(msg FileChange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for client := range s.Clients {
		client.WriteJSON(msg)
	}
}

func (s *Server) broadcastExcept(msg FileChange, skip *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	msg.Origin = "server"
	
	for client := range s.Clients {
		if client != skip {
			client.WriteJSON(msg)
		}
	}
}

func (s *Server) updateKnownFilesAndDirs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanDirRecursive(s.WatchDir, "")
}

func (s *Server) scanDirRecursive(basePath, relPath string) {
	fullPath := filepath.Join(basePath, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		itemRelPath := filepath.ToSlash(filepath.Join(relPath, entry.Name()))
		info, _ := entry.Info()
		
		if entry.IsDir() {
			if info != nil {
				s.knownDirs[itemRelPath] = info.ModTime()
			}
			s.scanDirRecursive(basePath, filepath.Join(relPath, entry.Name()))
		} else {
			if info != nil {
				s.knownFiles[itemRelPath] = info.ModTime()
			}
		}
	}
}

func (s *Server) cleanPendingMoves() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for range ticker.C {
		if s.shouldExit {
			return
		}
		
		s.mu.Lock()
		now := time.Now()
		for path, until := range s.pendingMoves {
			if now.After(until) {
				delete(s.pendingMoves, path)
			}
		}
		s.mu.Unlock()
	}
}