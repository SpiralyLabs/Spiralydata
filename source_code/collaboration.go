package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// 9. COLLABORATION MULTI-UTILISATEURS
// ============================================================================

// ============================================================================
// 9.1 GESTION MULTI-CLIENTS
// ============================================================================

// ClientInfo informations sur un client connecté
type ClientInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	IP           string            `json:"ip"`
	ConnectedAt  time.Time         `json:"connected_at"`
	LastActivity time.Time         `json:"last_activity"`
	OS           string            `json:"os"`
	Version      string            `json:"version"`
	Status       ClientStatus      `json:"status"`
	Role         UserRole          `json:"role"`
	Group        string            `json:"group"`
	Permissions  []string          `json:"permissions"`
	Stats        *ClientStats      `json:"stats"`
	Metadata     map[string]string `json:"metadata"`
	IsBanned     bool              `json:"is_banned"`
	BanReason    string            `json:"ban_reason,omitempty"`
	BanExpires   time.Time         `json:"ban_expires,omitempty"`
}

// ClientStatus statut du client
type ClientStatus int

const (
	ClientOnline ClientStatus = iota
	ClientAway
	ClientBusy
	ClientOffline
)

// String retourne le nom du statut
func (s ClientStatus) String() string {
	switch s {
	case ClientOnline:
		return "En ligne"
	case ClientAway:
		return "Absent"
	case ClientBusy:
		return "Occupé"
	case ClientOffline:
		return "Hors ligne"
	default:
		return "Inconnu"
	}
}

// Icon retourne l'icône du statut
func (s ClientStatus) Icon() string {
	switch s {
	case ClientOnline:
		return "🟢"
	case ClientAway:
		return "🟡"
	case ClientBusy:
		return "🔴"
	case ClientOffline:
		return "⚫"
	default:
		return "⚪"
	}
}

// ClientStats statistiques d'un client
type ClientStats struct {
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	FilesSent       int       `json:"files_sent"`
	FilesReceived   int       `json:"files_received"`
	SyncCount       int       `json:"sync_count"`
	LastSync        time.Time `json:"last_sync"`
	ErrorCount      int       `json:"error_count"`
	Uptime          time.Duration `json:"uptime"`
}

// ClientManager gère les clients connectés
type ClientManager struct {
	clients    map[string]*ClientInfo
	groups     map[string][]string // group -> client IDs
	mu         sync.RWMutex
	
	// Callbacks
	onClientJoin   []func(*ClientInfo)
	onClientLeave  []func(*ClientInfo)
	onClientUpdate []func(*ClientInfo)
}

// NewClientManager crée un gestionnaire de clients
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*ClientInfo),
		groups:  make(map[string][]string),
	}
}

// AddClient ajoute un client
func (cm *ClientManager) AddClient(client *ClientInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	client.ConnectedAt = time.Now()
	client.LastActivity = time.Now()
	client.Status = ClientOnline
	
	if client.Stats == nil {
		client.Stats = &ClientStats{}
	}
	
	cm.clients[client.ID] = client
	
	// Ajouter au groupe
	if client.Group != "" {
		cm.groups[client.Group] = append(cm.groups[client.Group], client.ID)
	}
	
	// Callbacks
	for _, cb := range cm.onClientJoin {
		go cb(client)
	}
	
	addLog(fmt.Sprintf("👤 Client connecté: %s (%s)", client.Name, client.IP))
}

// RemoveClient supprime un client
func (cm *ClientManager) RemoveClient(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	client, exists := cm.clients[clientID]
	if !exists {
		return
	}
	
	// Retirer du groupe
	if client.Group != "" {
		newGroup := make([]string, 0)
		for _, id := range cm.groups[client.Group] {
			if id != clientID {
				newGroup = append(newGroup, id)
			}
		}
		cm.groups[client.Group] = newGroup
	}
	
	delete(cm.clients, clientID)
	
	// Callbacks
	for _, cb := range cm.onClientLeave {
		go cb(client)
	}
	
	addLog(fmt.Sprintf("👤 Client déconnecté: %s", client.Name))
}

// GetClient retourne un client
func (cm *ClientManager) GetClient(clientID string) (*ClientInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	client, exists := cm.clients[clientID]
	return client, exists
}

// GetClients retourne tous les clients
func (cm *ClientManager) GetClients() []*ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var clients []*ClientInfo
	for _, c := range cm.clients {
		clients = append(clients, c)
	}
	return clients
}

// GetClientsByGroup retourne les clients d'un groupe
func (cm *ClientManager) GetClientsByGroup(group string) []*ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var clients []*ClientInfo
	for _, id := range cm.groups[group] {
		if client, exists := cm.clients[id]; exists {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetOnlineClients retourne les clients en ligne
func (cm *ClientManager) GetOnlineClients() []*ClientInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var clients []*ClientInfo
	for _, c := range cm.clients {
		if c.Status == ClientOnline && !c.IsBanned {
			clients = append(clients, c)
		}
	}
	return clients
}

// UpdateActivity met à jour l'activité d'un client
func (cm *ClientManager) UpdateActivity(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if client, exists := cm.clients[clientID]; exists {
		client.LastActivity = time.Now()
	}
}

// SetStatus définit le statut d'un client
func (cm *ClientManager) SetStatus(clientID string, status ClientStatus) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if client, exists := cm.clients[clientID]; exists {
		client.Status = status
		
		for _, cb := range cm.onClientUpdate {
			go cb(client)
		}
	}
}

// BanClient bannit un client
func (cm *ClientManager) BanClient(clientID, reason string, duration time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if client, exists := cm.clients[clientID]; exists {
		client.IsBanned = true
		client.BanReason = reason
		if duration > 0 {
			client.BanExpires = time.Now().Add(duration)
		}
		
		addLog(fmt.Sprintf("🚫 Client banni: %s - %s", client.Name, reason))
	}
}

// UnbanClient débannit un client
func (cm *ClientManager) UnbanClient(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if client, exists := cm.clients[clientID]; exists {
		client.IsBanned = false
		client.BanReason = ""
		client.BanExpires = time.Time{}
	}
}

// IsClientBanned vérifie si un client est banni
func (cm *ClientManager) IsClientBanned(clientID string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	client, exists := cm.clients[clientID]
	if !exists {
		return false
	}
	
	if !client.IsBanned {
		return false
	}
	
	// Vérifier l'expiration du ban
	if !client.BanExpires.IsZero() && time.Now().After(client.BanExpires) {
		client.IsBanned = false
		return false
	}
	
	return true
}

// GetGroups retourne tous les groupes
func (cm *ClientManager) GetGroups() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var groups []string
	for group := range cm.groups {
		groups = append(groups, group)
	}
	return groups
}

// AddToGroup ajoute un client à un groupe
func (cm *ClientManager) AddToGroup(clientID, group string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if client, exists := cm.clients[clientID]; exists {
		// Retirer de l'ancien groupe
		if client.Group != "" {
			newGroup := make([]string, 0)
			for _, id := range cm.groups[client.Group] {
				if id != clientID {
					newGroup = append(newGroup, id)
				}
			}
			cm.groups[client.Group] = newGroup
		}
		
		// Ajouter au nouveau groupe
		client.Group = group
		cm.groups[group] = append(cm.groups[group], clientID)
	}
}

// OnClientJoin callback quand un client rejoint
func (cm *ClientManager) OnClientJoin(cb func(*ClientInfo)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onClientJoin = append(cm.onClientJoin, cb)
}

// OnClientLeave callback quand un client part
func (cm *ClientManager) OnClientLeave(cb func(*ClientInfo)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onClientLeave = append(cm.onClientLeave, cb)
}

// ============================================================================
// 9.2 COMMUNICATION - CHAT
// ============================================================================

// ChatMessage message de chat
type ChatMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	FromName  string    `json:"from_name"`
	To        string    `json:"to,omitempty"`       // Vide = broadcast
	Channel   string    `json:"channel,omitempty"`  // Canal/Room
	Content   string    `json:"content"`
	Type      MessageType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ReadBy    []string  `json:"read_by,omitempty"`
	ReplyTo   string    `json:"reply_to,omitempty"`
	Mentions  []string  `json:"mentions,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}

// MessageType type de message
type MessageType int

const (
	MessageText MessageType = iota
	MessageSystem
	MessageNotification
	MessageFileComment
	MessageMention
)

// ChatManager gère le chat
type ChatManager struct {
	messages  []*ChatMessage
	channels  map[string][]*ChatMessage
	mu        sync.RWMutex
	maxMessages int
	msgCounter  int64
	
	// Callbacks
	onMessage []func(*ChatMessage)
}

// NewChatManager crée un gestionnaire de chat
func NewChatManager(maxMessages int) *ChatManager {
	return &ChatManager{
		messages:    make([]*ChatMessage, 0),
		channels:    make(map[string][]*ChatMessage),
		maxMessages: maxMessages,
	}
}

// SendMessage envoie un message
func (cm *ChatManager) SendMessage(from, fromName, content string, msgType MessageType) *ChatMessage {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.msgCounter++
	msg := &ChatMessage{
		ID:        fmt.Sprintf("MSG-%d-%d", time.Now().Unix(), cm.msgCounter),
		From:      from,
		FromName:  fromName,
		Content:   content,
		Type:      msgType,
		Timestamp: time.Now(),
		ReadBy:    []string{from},
	}
	
	// Détecter les mentions (@user)
	// Simple implémentation
	if msgType == MessageText {
		// Parse @mentions si nécessaire
	}
	
	cm.messages = append(cm.messages, msg)
	
	// Éviction
	if len(cm.messages) > cm.maxMessages {
		cm.messages = cm.messages[1:]
	}
	
	// Callbacks
	for _, cb := range cm.onMessage {
		go cb(msg)
	}
	
	return msg
}

// SendDirectMessage envoie un message privé
func (cm *ChatManager) SendDirectMessage(from, fromName, to, content string) *ChatMessage {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.msgCounter++
	msg := &ChatMessage{
		ID:        fmt.Sprintf("DM-%d-%d", time.Now().Unix(), cm.msgCounter),
		From:      from,
		FromName:  fromName,
		To:        to,
		Content:   content,
		Type:      MessageText,
		Timestamp: time.Now(),
		ReadBy:    []string{from},
	}
	
	cm.messages = append(cm.messages, msg)
	
	for _, cb := range cm.onMessage {
		go cb(msg)
	}
	
	return msg
}

// SendToChannel envoie un message à un canal
func (cm *ChatManager) SendToChannel(from, fromName, channel, content string) *ChatMessage {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.msgCounter++
	msg := &ChatMessage{
		ID:        fmt.Sprintf("CH-%d-%d", time.Now().Unix(), cm.msgCounter),
		From:      from,
		FromName:  fromName,
		Channel:   channel,
		Content:   content,
		Type:      MessageText,
		Timestamp: time.Now(),
		ReadBy:    []string{from},
	}
	
	cm.channels[channel] = append(cm.channels[channel], msg)
	
	// Éviction par canal
	if len(cm.channels[channel]) > cm.maxMessages {
		cm.channels[channel] = cm.channels[channel][1:]
	}
	
	for _, cb := range cm.onMessage {
		go cb(msg)
	}
	
	return msg
}

// GetMessages retourne les messages récents
func (cm *ChatManager) GetMessages(limit int) []*ChatMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if limit <= 0 || limit > len(cm.messages) {
		limit = len(cm.messages)
	}
	
	start := len(cm.messages) - limit
	result := make([]*ChatMessage, limit)
	copy(result, cm.messages[start:])
	return result
}

// GetChannelMessages retourne les messages d'un canal
func (cm *ChatManager) GetChannelMessages(channel string, limit int) []*ChatMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	msgs := cm.channels[channel]
	if limit <= 0 || limit > len(msgs) {
		limit = len(msgs)
	}
	
	start := len(msgs) - limit
	result := make([]*ChatMessage, limit)
	copy(result, msgs[start:])
	return result
}

// GetDirectMessages retourne les messages privés entre deux utilisateurs
func (cm *ChatManager) GetDirectMessages(user1, user2 string, limit int) []*ChatMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var result []*ChatMessage
	
	for i := len(cm.messages) - 1; i >= 0 && (limit <= 0 || len(result) < limit); i-- {
		msg := cm.messages[i]
		if (msg.From == user1 && msg.To == user2) || (msg.From == user2 && msg.To == user1) {
			result = append([]*ChatMessage{msg}, result...)
		}
	}
	
	return result
}

// MarkAsRead marque un message comme lu
func (cm *ChatManager) MarkAsRead(messageID, userID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for _, msg := range cm.messages {
		if msg.ID == messageID {
			for _, id := range msg.ReadBy {
				if id == userID {
					return // Déjà lu
				}
			}
			msg.ReadBy = append(msg.ReadBy, userID)
			return
		}
	}
}

// GetChannels retourne les canaux
func (cm *ChatManager) GetChannels() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var channels []string
	for ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

// CreateChannel crée un canal
func (cm *ChatManager) CreateChannel(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if _, exists := cm.channels[name]; !exists {
		cm.channels[name] = make([]*ChatMessage, 0)
	}
}

// OnMessage callback pour les messages
func (cm *ChatManager) OnMessage(cb func(*ChatMessage)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onMessage = append(cm.onMessage, cb)
}

// ============================================================================
// 9.3 COLLABORATION - FILE LOCKING
// ============================================================================

// FileLock verrouillage de fichier
type FileLock struct {
	Path       string    `json:"path"`
	LockedBy   string    `json:"locked_by"`
	LockedByName string  `json:"locked_by_name"`
	LockedAt   time.Time `json:"locked_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Reason     string    `json:"reason,omitempty"`
}

// FileLockManager gère les verrouillages
type FileLockManager struct {
	locks map[string]*FileLock
	mu    sync.RWMutex
	defaultTTL time.Duration
}

// NewFileLockManager crée un gestionnaire de verrous
func NewFileLockManager(defaultTTL time.Duration) *FileLockManager {
	flm := &FileLockManager{
		locks:      make(map[string]*FileLock),
		defaultTTL: defaultTTL,
	}
	
	// Nettoyage périodique
	go flm.cleanupLoop()
	
	return flm
}

// Lock verrouille un fichier
func (flm *FileLockManager) Lock(path, userID, userName, reason string) (*FileLock, error) {
	flm.mu.Lock()
	defer flm.mu.Unlock()
	
	// Vérifier si déjà verrouillé
	if lock, exists := flm.locks[path]; exists {
		if time.Now().Before(lock.ExpiresAt) && lock.LockedBy != userID {
			return nil, fmt.Errorf("fichier verrouillé par %s", lock.LockedByName)
		}
	}
	
	lock := &FileLock{
		Path:         path,
		LockedBy:     userID,
		LockedByName: userName,
		LockedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(flm.defaultTTL),
		Reason:       reason,
	}
	
	flm.locks[path] = lock
	addLog(fmt.Sprintf("🔒 Fichier verrouillé: %s par %s", path, userName))
	
	return lock, nil
}

// Unlock déverrouille un fichier
func (flm *FileLockManager) Unlock(path, userID string) error {
	flm.mu.Lock()
	defer flm.mu.Unlock()
	
	lock, exists := flm.locks[path]
	if !exists {
		return nil
	}
	
	if lock.LockedBy != userID {
		return fmt.Errorf("seul %s peut déverrouiller ce fichier", lock.LockedByName)
	}
	
	delete(flm.locks, path)
	addLog(fmt.Sprintf("🔓 Fichier déverrouillé: %s", path))
	
	return nil
}

// ForceUnlock déverrouille de force (admin)
func (flm *FileLockManager) ForceUnlock(path string) {
	flm.mu.Lock()
	defer flm.mu.Unlock()
	
	delete(flm.locks, path)
}

// IsLocked vérifie si un fichier est verrouillé
func (flm *FileLockManager) IsLocked(path string) bool {
	flm.mu.RLock()
	defer flm.mu.RUnlock()
	
	lock, exists := flm.locks[path]
	if !exists {
		return false
	}
	
	return time.Now().Before(lock.ExpiresAt)
}

// GetLock retourne le verrou d'un fichier
func (flm *FileLockManager) GetLock(path string) (*FileLock, bool) {
	flm.mu.RLock()
	defer flm.mu.RUnlock()
	
	lock, exists := flm.locks[path]
	if !exists || time.Now().After(lock.ExpiresAt) {
		return nil, false
	}
	
	return lock, true
}

// GetAllLocks retourne tous les verrous actifs
func (flm *FileLockManager) GetAllLocks() []*FileLock {
	flm.mu.RLock()
	defer flm.mu.RUnlock()
	
	var locks []*FileLock
	now := time.Now()
	
	for _, lock := range flm.locks {
		if now.Before(lock.ExpiresAt) {
			locks = append(locks, lock)
		}
	}
	
	return locks
}

// RefreshLock renouvelle un verrou
func (flm *FileLockManager) RefreshLock(path, userID string) error {
	flm.mu.Lock()
	defer flm.mu.Unlock()
	
	lock, exists := flm.locks[path]
	if !exists {
		return fmt.Errorf("verrou non trouvé")
	}
	
	if lock.LockedBy != userID {
		return fmt.Errorf("non autorisé")
	}
	
	lock.ExpiresAt = time.Now().Add(flm.defaultTTL)
	return nil
}

func (flm *FileLockManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		flm.mu.Lock()
		now := time.Now()
		for path, lock := range flm.locks {
			if now.After(lock.ExpiresAt) {
				delete(flm.locks, path)
			}
		}
		flm.mu.Unlock()
	}
}

// ============================================================================
// NOTIFICATIONS
// ============================================================================

// Notification notification
type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	UserID    string    `json:"user_id,omitempty"` // Destinataire (vide = tous)
	FromUser  string    `json:"from_user,omitempty"`
	FilePath  string    `json:"file_path,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
	Action    string    `json:"action,omitempty"` // URL ou action à effectuer
}

// NotificationManager gère les notifications
type NotificationManager struct {
	notifications []*Notification
	mu            sync.RWMutex
	maxNotifs     int
	notifCounter  int64
	
	onNotification []func(*Notification)
}

// NewNotificationManager crée un gestionnaire de notifications
func NewNotificationManager(maxNotifs int) *NotificationManager {
	return &NotificationManager{
		notifications: make([]*Notification, 0),
		maxNotifs:     maxNotifs,
	}
}

// Notify envoie une notification
func (nm *NotificationManager) Notify(notifType, title, message, userID string) *Notification {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	nm.notifCounter++
	notif := &Notification{
		ID:        fmt.Sprintf("NOTIF-%d", nm.notifCounter),
		Type:      notifType,
		Title:     title,
		Message:   message,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	
	nm.notifications = append(nm.notifications, notif)
	
	if len(nm.notifications) > nm.maxNotifs {
		nm.notifications = nm.notifications[1:]
	}
	
	for _, cb := range nm.onNotification {
		go cb(notif)
	}
	
	return notif
}

// NotifyFileChange notifie un changement de fichier
func (nm *NotificationManager) NotifyFileChange(action, filePath, fromUser string) {
	nm.Notify("file_change", "Fichier modifié", 
		fmt.Sprintf("%s a %s %s", fromUser, action, filePath), "")
}

// GetNotifications retourne les notifications d'un utilisateur
func (nm *NotificationManager) GetNotifications(userID string, unreadOnly bool) []*Notification {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	var result []*Notification
	
	for _, n := range nm.notifications {
		if n.UserID != "" && n.UserID != userID {
			continue
		}
		if unreadOnly && n.Read {
			continue
		}
		result = append(result, n)
	}
	
	return result
}

// MarkAsRead marque une notification comme lue
func (nm *NotificationManager) MarkAsRead(notifID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	for _, n := range nm.notifications {
		if n.ID == notifID {
			n.Read = true
			return
		}
	}
}

// OnNotification callback pour les notifications
func (nm *NotificationManager) OnNotification(cb func(*Notification)) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.onNotification = append(nm.onNotification, cb)
}

// ============================================================================
// BROADCAST
// ============================================================================

// BroadcastMessage message broadcast
type BroadcastMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	From    string          `json:"from,omitempty"`
	To      []string        `json:"to,omitempty"` // Vide = tous
}

// Broadcaster diffuse des messages
type Broadcaster struct {
	listeners []func(BroadcastMessage)
	mu        sync.RWMutex
}

// NewBroadcaster crée un broadcaster
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		listeners: make([]func(BroadcastMessage), 0),
	}
}

// Broadcast diffuse un message
func (b *Broadcaster) Broadcast(msgType string, payload interface{}, to []string) {
	data, _ := json.Marshal(payload)
	
	msg := BroadcastMessage{
		Type:    msgType,
		Payload: data,
		To:      to,
	}
	
	b.mu.RLock()
	listeners := b.listeners
	b.mu.RUnlock()
	
	for _, listener := range listeners {
		go listener(msg)
	}
}

// Subscribe s'abonne aux broadcasts
func (b *Broadcaster) Subscribe(cb func(BroadcastMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, cb)
}

// ============================================================================
// GLOBAL INSTANCES
// ============================================================================

var (
	globalClientManager      = NewClientManager()
	globalChatManager        = NewChatManager(1000)
	globalFileLockManager    = NewFileLockManager(30 * time.Minute)
	globalNotificationMgr    = NewNotificationManager(500)
	globalBroadcaster        = NewBroadcaster()
)

// GetClientManager retourne le gestionnaire de clients
func GetClientManager() *ClientManager { return globalClientManager }

// GetChatManager retourne le gestionnaire de chat
func GetChatManager() *ChatManager { return globalChatManager }

// GetFileLockManager retourne le gestionnaire de verrous
func GetFileLockManager() *FileLockManager { return globalFileLockManager }

// GetNotificationManager retourne le gestionnaire de notifications
func GetNotificationManager() *NotificationManager { return globalNotificationMgr }

// GetBroadcaster retourne le broadcaster
func GetBroadcaster() *Broadcaster { return globalBroadcaster }
