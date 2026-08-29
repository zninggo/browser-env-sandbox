// Package dashboard provides a web-based management UI for browser-env-sandbox.
// Serves a single-page dashboard at /dashboard that shows active sessions,
// fingerprints, cookies, network requests, and console output.
package dashboard

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

// DashboardServer serves the web UI and JSON API for management.
type DashboardServer struct {
	addr     string
	provider DashboardProvider
	mu       sync.Mutex
}

// DashboardProvider provides data for the dashboard.
type DashboardProvider interface {
	ListSessions() []SessionSummary
	GetSession(id string) (*SessionDetail, error)
	CloseSession(id string) error
}

// SessionSummary is a brief session listing for the dashboard table.
type SessionSummary struct {
	ID        string `json:"id"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	UA        string `json:"ua"`
	Proxy     string `json:"proxy"`
	CreatedAt string `json:"created_at"`
}

// SessionDetail is full session info for the detail view.
type SessionDetail struct {
	SessionSummary
	Fingerprint map[string]interface{} `json:"fingerprint"`
	Cookies     string                  `json:"cookies"`
	Location    string                  `json:"location"`
}

// New creates a dashboard server.
func New(addr string, provider DashboardProvider) *DashboardServer {
	if addr == "" {
		addr = ":19822"
	}
	return &DashboardServer{addr: addr, provider: provider}
}

// Start begins serving the dashboard.
func (d *DashboardServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/dashboard", d.handleDashboard)
	mux.HandleFunc("/api/dashboard/sessions", d.handleSessions)
	mux.HandleFunc("/api/dashboard/session/", d.handleSessionDetail)
	mux.HandleFunc("/api/dashboard/health", d.handleHealth)

	ln, err := net.Listen("tcp", d.addr) //nolint:staticcheck
	if err != nil {
		return err
	}
	log.Printf("[dashboard] listening on %s", d.addr)
	go http.Serve(ln, mux)
	return nil
}

// /dashboard — serves the SPA HTML
func (d *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

// /api/dashboard/sessions — JSON list of sessions
func (d *DashboardServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sessions := d.provider.ListSessions()
	if sessions == nil {
		sessions = []SessionSummary{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// /api/dashboard/session/{id} — JSON detail of a session
func (d *DashboardServer) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, `{"error":"missing session id"}`, 400)
		return
	}
	id := parts[4]
	detail, err := d.provider.GetSession(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(detail)
}

// /api/dashboard/health — health check
func (d *DashboardServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "0.2.0",
	})
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>browser-env-sandbox Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f1117; color: #e0e0e0; }
  .header { background: #1a1d27; padding: 16px 24px; border-bottom: 1px solid #2a2d37; display: flex; justify-content: space-between; align-items: center; }
  .header h1 { font-size: 18px; color: #8b5cf6; }
  .header .status { font-size: 13px; color: #4ade80; }
  .container { max-width: 1400px; margin: 0 auto; padding: 24px; }
  .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
  .stat-card { background: #1a1d27; border-radius: 8px; padding: 20px; border: 1px solid #2a2d37; }
  .stat-card .label { font-size: 12px; color: #888; text-transform: uppercase; letter-spacing: 1px; }
  .stat-card .value { font-size: 28px; font-weight: 700; color: #e0e0e0; margin-top: 4px; }
  table { width: 100%; background: #1a1d27; border-radius: 8px; overflow: hidden; border: 1px solid #2a2d37; }
  th { background: #222530; padding: 12px 16px; text-align: left; font-size: 12px; text-transform: uppercase; letter-spacing: 1px; color: #888; }
  td { padding: 12px 16px; border-top: 1px solid #2a2d37; font-size: 14px; }
  tr:hover td { background: #1e2130; cursor: pointer; }
  .empty { text-align: center; padding: 48px; color: #555; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; }
  .badge-chrome { background: #4285f4; color: white; }
  .badge-firefox { background: #ff7139; color: white; }
  .badge-safari { background: #1ba0d7; color: white; }
  .refresh-btn { background: #8b5cf6; color: white; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; font-size: 13px; }
  .refresh-btn:hover { background: #7c3aed; }
  #detail-panel { display: none; margin-top: 24px; background: #1a1d27; border-radius: 8px; padding: 24px; border: 1px solid #2a2d37; }
  #detail-panel h2 { color: #8b5cf6; margin-bottom: 16px; }
  #detail-panel pre { background: #0f1117; padding: 16px; border-radius: 6px; overflow-x: auto; font-size: 13px; color: #a0a0a0; }
</style>
</head>
<body>
<div class="header">
  <h1>🌐 browser-env-sandbox</h1>
  <div class="status" id="status">● Connected</div>
  <button class="refresh-btn" onclick="loadSessions()">Refresh</button>
</div>
<div class="container">
  <div class="stats">
    <div class="stat-card"><div class="label">Active Sessions</div><div class="value" id="stat-sessions">0</div></div>
    <div class="stat-card"><div class="label">Chrome Versions</div><div class="value" id="stat-versions">148-152</div></div>
    <div class="stat-card"><div class="label">API Endpoint</div><div class="value" id="stat-api">:19821</div></div>
    <div class="stat-card"><div class="label">CDP Debug</div><div class="value" id="stat-cdp">:9223</div></div>
  </div>
  <table>
    <thead><tr><th>Session ID</th><th>Browser</th><th>OS</th><th>User Agent</th><th>Proxy</th><th>Created</th></tr></thead>
    <tbody id="sessions-body">
      <tr><td colspan="6" class="empty">Loading...</td></tr>
    </tbody>
  </table>
  <div id="detail-panel">
    <h2>Session Detail</h2>
    <pre id="detail-content"></pre>
  </div>
</div>
<script>
async function loadSessions() {
  try {
    const resp = await fetch('/api/dashboard/sessions');
    const data = await resp.json();
    document.getElementById('stat-sessions').textContent = data.count;
    const body = document.getElementById('sessions-body');
    if (data.sessions.length === 0) {
      body.innerHTML = '<tr><td colspan="6" class="empty">No active sessions</td></tr>';
      return;
    }
    body.innerHTML = data.sessions.map(s => '<tr onclick="showDetail(\''+s.id+'\')">' +
      '<td style="font-family:monospace;font-size:12px">'+s.id.substring(0,16)+'...</td>' +
      '<td><span class="badge badge-'+s.browser+'">'+s.browser+'</span></td>' +
      '<td>'+s.os+'</td>' +
      '<td style="font-size:12px;max-width:300px;overflow:hidden;text-overflow:ellipsis">'+s.ua+'</td>' +
      '<td>'+(s.proxy||'—')+'</td>' +
      '<td style="font-size:12px">'+s.created_at+'</td>' +
      '</tr>').join('');
  } catch(e) {
    document.getElementById('status').textContent = '● Error: ' + e.message;
  }
}
async function showDetail(id) {
  try {
    const resp = await fetch('/api/dashboard/session/'+id);
    const data = await resp.json();
    document.getElementById('detail-panel').style.display = 'block';
    document.getElementById('detail-content').textContent = JSON.stringify(data, null, 2);
  } catch(e) { console.error(e); }
}
loadSessions();
setInterval(loadSessions, 5000);
</script>
</body>
</html>`
