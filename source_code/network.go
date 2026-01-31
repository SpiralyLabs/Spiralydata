package main

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// 13. RÉSEAU & CONNECTIVITÉ
// ============================================================================

// ConnectionState état de connexion
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
)

// String retourne le nom
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Déconnecté"
	case StateConnecting:
		return "Connexion..."
	case StateConnected:
		return "Connecté"
	case StateReconnecting:
		return "Reconnexion..."
	case StateFailed:
		return "Échec"
	default:
		return "Inconnu"
	}
}

// Icon retourne l'icône
func (s ConnectionState) Icon() string {
	switch s {
	case StateDisconnected:
		return "⚫"
	case StateConnecting:
		return "🟡"
	case StateConnected:
		return "🟢"
	case StateReconnecting:
		return "🟠"
	case StateFailed:
		return "🔴"
	default:
		return "⚪"
	}
}

// NetworkConfig configuration réseau
type NetworkConfig struct {
	// Reconnexion
	AutoReconnect     bool          `json:"auto_reconnect"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	MaxRetryDelay     time.Duration `json:"max_retry_delay"`
	RetryBackoff      float64       `json:"retry_backoff"`
	
	// Keep-alive
	KeepAliveEnabled  bool          `json:"keep_alive_enabled"`
	KeepAliveInterval time.Duration `json:"keep_alive_interval"`
	KeepAliveTimeout  time.Duration `json:"keep_alive_timeout"`
	
	// Timeouts
	ConnectTimeout    time.Duration `json:"connect_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	
	// Bande passante
	MaxBandwidth      int64         `json:"max_bandwidth"` // bytes/sec, 0 = illimité
}

// NewNetworkConfig crée une config par défaut
func NewNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		AutoReconnect:     true,
		MaxRetries:        10,
		RetryDelay:        time.Second,
		MaxRetryDelay:     60 * time.Second,
		RetryBackoff:      2.0,
		KeepAliveEnabled:  true,
		KeepAliveInterval: 30 * time.Second,
		KeepAliveTimeout:  10 * time.Second,
		ConnectTimeout:    30 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxBandwidth:      0,
	}
}

// ConnectionManager gère la connexion
type ConnectionManager struct {
	config     *NetworkConfig
	state      ConnectionState
	mu         sync.RWMutex
	
	// Stats
	connectTime     time.Time
	lastActivity    time.Time
	reconnectCount  int
	totalSent       int64
	totalReceived   int64
	
	// Keep-alive
	keepAliveStop   chan bool
	keepAliveActive bool
	
	// Qualité
	latency         time.Duration
	packetLoss      float64
	bandwidth       int64
	
	// Callbacks
	onStateChange   []func(ConnectionState)
	onDisconnect    []func()
	onReconnect     []func()
}

// NewConnectionManager crée un gestionnaire
func NewConnectionManager(config *NetworkConfig) *ConnectionManager {
	return &ConnectionManager{
		config:        config,
		state:         StateDisconnected,
		keepAliveStop: make(chan bool),
	}
}

// GetState retourne l'état
func (cm *ConnectionManager) GetState() ConnectionState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state
}

// SetState définit l'état
func (cm *ConnectionManager) SetState(state ConnectionState) {
	cm.mu.Lock()
	oldState := cm.state
	cm.state = state
	callbacks := cm.onStateChange
	cm.mu.Unlock()
	
	if oldState != state {
		for _, cb := range callbacks {
			go cb(state)
		}
	}
}

// MarkConnected marque comme connecté
func (cm *ConnectionManager) MarkConnected() {
	cm.mu.Lock()
	cm.state = StateConnected
	cm.connectTime = time.Now()
	cm.lastActivity = time.Now()
	cm.mu.Unlock()
	
	if cm.config.KeepAliveEnabled {
		cm.startKeepAlive()
	}
	
	cm.SetState(StateConnected)
}

// MarkDisconnected marque comme déconnecté
func (cm *ConnectionManager) MarkDisconnected() {
	cm.stopKeepAlive()
	
	cm.mu.Lock()
	callbacks := cm.onDisconnect
	cm.mu.Unlock()
	
	for _, cb := range callbacks {
		go cb()
	}
	
	cm.SetState(StateDisconnected)
}

// RecordActivity enregistre une activité
func (cm *ConnectionManager) RecordActivity() {
	cm.mu.Lock()
	cm.lastActivity = time.Now()
	cm.mu.Unlock()
}

// RecordSent enregistre des données envoyées
func (cm *ConnectionManager) RecordSent(bytes int64) {
	atomic.AddInt64(&cm.totalSent, bytes)
	cm.RecordActivity()
}

// RecordReceived enregistre des données reçues
func (cm *ConnectionManager) RecordReceived(bytes int64) {
	atomic.AddInt64(&cm.totalReceived, bytes)
	cm.RecordActivity()
}

// GetStats retourne les stats
func (cm *ConnectionManager) GetStats() ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var uptime time.Duration
	if cm.state == StateConnected {
		uptime = time.Since(cm.connectTime)
	}
	
	return ConnectionStats{
		State:          cm.state,
		Uptime:         uptime,
		ReconnectCount: cm.reconnectCount,
		TotalSent:      atomic.LoadInt64(&cm.totalSent),
		TotalReceived:  atomic.LoadInt64(&cm.totalReceived),
		LastActivity:   cm.lastActivity,
		Latency:        cm.latency,
		PacketLoss:     cm.packetLoss,
		Bandwidth:      cm.bandwidth,
	}
}

// ConnectionStats statistiques
type ConnectionStats struct {
	State          ConnectionState
	Uptime         time.Duration
	ReconnectCount int
	TotalSent      int64
	TotalReceived  int64
	LastActivity   time.Time
	Latency        time.Duration
	PacketLoss     float64
	Bandwidth      int64
}

// OnStateChange callback de changement d'état
func (cm *ConnectionManager) OnStateChange(cb func(ConnectionState)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onStateChange = append(cm.onStateChange, cb)
}

// OnDisconnect callback de déconnexion
func (cm *ConnectionManager) OnDisconnect(cb func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onDisconnect = append(cm.onDisconnect, cb)
}

// OnReconnect callback de reconnexion
func (cm *ConnectionManager) OnReconnect(cb func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onReconnect = append(cm.onReconnect, cb)
}

// ============================================================================
// RECONNEXION AUTOMATIQUE
// ============================================================================

// ReconnectStrategy stratégie de reconnexion
type ReconnectStrategy struct {
	config       *NetworkConfig
	attempts     int
	lastAttempt  time.Time
	currentDelay time.Duration
}

// NewReconnectStrategy crée une stratégie
func NewReconnectStrategy(config *NetworkConfig) *ReconnectStrategy {
	return &ReconnectStrategy{
		config:       config,
		currentDelay: config.RetryDelay,
	}
}

// ShouldRetry vérifie si on doit réessayer
func (rs *ReconnectStrategy) ShouldRetry() bool {
	if !rs.config.AutoReconnect {
		return false
	}
	if rs.config.MaxRetries > 0 && rs.attempts >= rs.config.MaxRetries {
		return false
	}
	return true
}

// GetDelay retourne le délai
func (rs *ReconnectStrategy) GetDelay() time.Duration {
	return rs.currentDelay
}

// RecordAttempt enregistre une tentative
func (rs *ReconnectStrategy) RecordAttempt(success bool) {
	rs.attempts++
	rs.lastAttempt = time.Now()
	
	if success {
		rs.Reset()
	} else {
		rs.currentDelay = time.Duration(float64(rs.currentDelay) * rs.config.RetryBackoff)
		if rs.currentDelay > rs.config.MaxRetryDelay {
			rs.currentDelay = rs.config.MaxRetryDelay
		}
	}
}

// Reset réinitialise
func (rs *ReconnectStrategy) Reset() {
	rs.attempts = 0
	rs.currentDelay = rs.config.RetryDelay
}

// GetAttempts retourne le nombre de tentatives
func (rs *ReconnectStrategy) GetAttempts() int {
	return rs.attempts
}

// ============================================================================
// KEEP-ALIVE
// ============================================================================

func (cm *ConnectionManager) startKeepAlive() {
	cm.mu.Lock()
	if cm.keepAliveActive {
		cm.mu.Unlock()
		return
	}
	cm.keepAliveActive = true
	cm.keepAliveStop = make(chan bool)
	cm.mu.Unlock()
	
	go func() {
		ticker := time.NewTicker(cm.config.KeepAliveInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-cm.keepAliveStop:
				return
			case <-ticker.C:
				cm.mu.RLock()
				lastActivity := cm.lastActivity
				cm.mu.RUnlock()
				
				if time.Since(lastActivity) > cm.config.KeepAliveTimeout*2 {
					addLog("⚠️ Timeout keep-alive détecté")
					cm.MarkDisconnected()
					return
				}
			}
		}
	}()
}

func (cm *ConnectionManager) stopKeepAlive() {
	cm.mu.Lock()
	if cm.keepAliveActive {
		close(cm.keepAliveStop)
		cm.keepAliveActive = false
	}
	cm.mu.Unlock()
}

// ============================================================================
// QUALITÉ DE CONNEXION
// ============================================================================

// ConnectionQuality qualité
type ConnectionQuality int

const (
	QualityExcellent ConnectionQuality = iota
	QualityGood
	QualityFair
	QualityPoor
	QualityBad
)

// String retourne le nom
func (q ConnectionQuality) String() string {
	switch q {
	case QualityExcellent:
		return "Excellente"
	case QualityGood:
		return "Bonne"
	case QualityFair:
		return "Moyenne"
	case QualityPoor:
		return "Faible"
	case QualityBad:
		return "Mauvaise"
	default:
		return "Inconnue"
	}
}

// Icon retourne l'icône
func (q ConnectionQuality) Icon() string {
	switch q {
	case QualityExcellent:
		return "📶"
	case QualityGood:
		return "📶"
	case QualityFair:
		return "📶"
	case QualityPoor:
		return "📵"
	default:
		return "❌"
	}
}

// MeasureLatency mesure la latence
func MeasureLatency(host string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return 0, err
	}
	
	latency := time.Since(start)
	conn.Close()
	
	return latency, nil
}

// GetConnectionQuality évalue la qualité
func (cm *ConnectionManager) GetConnectionQuality() ConnectionQuality {
	cm.mu.RLock()
	latency := cm.latency
	packetLoss := cm.packetLoss
	cm.mu.RUnlock()
	
	if packetLoss > 10 {
		return QualityBad
	}
	if packetLoss > 5 || latency > 500*time.Millisecond {
		return QualityPoor
	}
	if packetLoss > 2 || latency > 200*time.Millisecond {
		return QualityFair
	}
	if latency > 100*time.Millisecond {
		return QualityGood
	}
	return QualityExcellent
}

// UpdateLatency met à jour la latence
func (cm *ConnectionManager) UpdateLatency(latency time.Duration) {
	cm.mu.Lock()
	cm.latency = (cm.latency + latency) / 2
	cm.mu.Unlock()
}

// UpdateBandwidth met à jour la bande passante
func (cm *ConnectionManager) UpdateBandwidth(bytes int64, duration time.Duration) {
	if duration > 0 {
		bandwidth := int64(float64(bytes) / duration.Seconds())
		cm.mu.Lock()
		cm.bandwidth = (cm.bandwidth + bandwidth) / 2
		cm.mu.Unlock()
	}
}

// ============================================================================
// BANDWIDTH LIMITER
// ============================================================================

// BandwidthLimiter limite la bande passante
type BandwidthLimiter struct {
	maxBytesPerSecond int64
	bytesThisSecond   int64
	lastReset         time.Time
	mu                sync.Mutex
}

// NewBandwidthLimiter crée un limiteur
func NewBandwidthLimiter(maxBytesPerSecond int64) *BandwidthLimiter {
	return &BandwidthLimiter{
		maxBytesPerSecond: maxBytesPerSecond,
		lastReset:         time.Now(),
	}
}

// SetLimit définit la limite
func (bl *BandwidthLimiter) SetLimit(maxBytesPerSecond int64) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.maxBytesPerSecond = maxBytesPerSecond
}

// WaitForBandwidth attend de la bande passante
func (bl *BandwidthLimiter) WaitForBandwidth(bytes int64) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	
	if bl.maxBytesPerSecond <= 0 {
		return
	}
	
	if time.Since(bl.lastReset) >= time.Second {
		bl.bytesThisSecond = 0
		bl.lastReset = time.Now()
	}
	
	for bl.bytesThisSecond+bytes > bl.maxBytesPerSecond {
		remaining := time.Second - time.Since(bl.lastReset)
		if remaining > 0 {
			bl.mu.Unlock()
			time.Sleep(remaining)
			bl.mu.Lock()
		}
		bl.bytesThisSecond = 0
		bl.lastReset = time.Now()
	}
	
	bl.bytesThisSecond += bytes
}

// GetCurrentUsage retourne l'utilisation
func (bl *BandwidthLimiter) GetCurrentUsage() int64 {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	
	if time.Since(bl.lastReset) >= time.Second {
		return 0
	}
	return bl.bytesThisSecond
}

// ============================================================================
// HEALTH CHECK
// ============================================================================

// HealthStatus statut de santé
type HealthStatus struct {
	Healthy     bool      `json:"healthy"`
	LastCheck   time.Time `json:"last_check"`
	Latency     time.Duration `json:"latency"`
	ErrorCount  int       `json:"error_count"`
	Message     string    `json:"message"`
}

// HealthChecker vérifie la santé
type HealthChecker struct {
	host        string
	interval    time.Duration
	timeout     time.Duration
	status      *HealthStatus
	mu          sync.RWMutex
	stopChan    chan bool
	running     bool
	onUnhealthy []func(*HealthStatus)
}

// NewHealthChecker crée un vérificateur
func NewHealthChecker(host string, interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		host:     host,
		interval: interval,
		timeout:  timeout,
		status:   &HealthStatus{Healthy: true},
		stopChan: make(chan bool),
	}
}

// Start démarre les vérifications
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.stopChan = make(chan bool)
	hc.mu.Unlock()
	
	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-hc.stopChan:
				return
			case <-ticker.C:
				hc.check()
			}
		}
	}()
}

// Stop arrête les vérifications
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if hc.running {
		close(hc.stopChan)
		hc.running = false
	}
	hc.mu.Unlock()
}

func (hc *HealthChecker) check() {
	latency, err := MeasureLatency(hc.host, hc.timeout)
	
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.status.LastCheck = time.Now()
	hc.status.Latency = latency
	
	if err != nil {
		hc.status.ErrorCount++
		hc.status.Message = err.Error()
		
		if hc.status.ErrorCount >= 3 {
			hc.status.Healthy = false
			
			for _, cb := range hc.onUnhealthy {
				go cb(hc.status)
			}
		}
	} else {
		hc.status.Healthy = true
		hc.status.ErrorCount = 0
		hc.status.Message = fmt.Sprintf("OK - %v", latency)
	}
}

// GetStatus retourne le statut
func (hc *HealthChecker) GetStatus() *HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.status
}

// OnUnhealthy callback quand pas healthy
func (hc *HealthChecker) OnUnhealthy(cb func(*HealthStatus)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onUnhealthy = append(hc.onUnhealthy, cb)
}

// ============================================================================
// GLOBAL INSTANCES
// ============================================================================

var (
	globalNetworkConfig    = NewNetworkConfig()
	globalConnectionMgr    *ConnectionManager
	globalBandwidthLimiter *BandwidthLimiter
)

func init() {
	globalConnectionMgr = NewConnectionManager(globalNetworkConfig)
	globalBandwidthLimiter = NewBandwidthLimiter(0)
}

// GetNetworkConfig retourne la config
func GetNetworkConfig() *NetworkConfig { return globalNetworkConfig }

// GetConnectionManager retourne le gestionnaire
func GetConnectionManager() *ConnectionManager { return globalConnectionMgr }

// GetBandwidthLimiter retourne le limiteur
func GetBandwidthLimiter() *BandwidthLimiter { return globalBandwidthLimiter }
