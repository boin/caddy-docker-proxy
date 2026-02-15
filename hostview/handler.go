package hostview

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Handler serves the host status page and data API
type Handler struct {
	templatePath string
	getConfig    func() []byte
}

// NewHandler creates a new host status handler
func NewHandler(templatePath string, getConfig func() []byte) *Handler {
	return &Handler{
		templatePath: templatePath,
		getConfig:    getConfig,
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle data API endpoint
	if strings.HasSuffix(r.URL.Path, "/data") {
		h.serveData(w, r)
		return
	}

	// Handle page request
	h.servePage(w, r)
}

// servePage serves the HTML template
func (h *Handler) servePage(w http.ResponseWriter, r *http.Request) {
	// Read template file on each request (supports hot reload)
	content, err := os.ReadFile(h.templatePath)
	if err != nil {
		http.Error(w, "Template not found: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// serveData serves the host list as JSON
func (h *Handler) serveData(w http.ResponseWriter, r *http.Request) {
	configJSON := h.getConfig()
	hostList := ParseHostList(configJSON)

	response := map[string]any{
		"hosts":     hostList,
		"updatedAt": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DefaultTemplate returns the default HTML template content
// This can be used when no external template is specified
func DefaultTemplate() string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Caddy 主机列表</title>
  <style>
    :root {
      --bg-primary: #1d232a;
      --bg-secondary: #2a323c;
      --text-primary: #a6adba;
      --text-accent: #7480ff;
      --border-color: #3d4451;
    }
    * { box-sizing: border-box; }
    body {
      font-family: system-ui, -apple-system, sans-serif;
      background: var(--bg-primary);
      color: var(--text-primary);
      margin: 0;
      padding: 1rem;
      line-height: 1.5;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header { 
      display: flex; 
      justify-content: space-between; 
      align-items: center; 
      margin-bottom: 1.5rem;
      flex-wrap: wrap;
      gap: 1rem;
    }
    .header h1 { margin: 0; font-size: 1.5rem; }
    .toolbar { display: flex; gap: 0.5rem; align-items: center; }
    .search { 
      padding: 0.5rem 1rem; 
      border-radius: 0.5rem; 
      border: 1px solid var(--border-color); 
      background: var(--bg-secondary); 
      color: var(--text-primary);
      font-size: 0.875rem;
      width: 250px;
    }
    .search:focus { outline: 2px solid var(--text-accent); outline-offset: 1px; }
    .btn {
      padding: 0.5rem 1rem;
      border-radius: 0.5rem;
      border: 1px solid var(--border-color);
      background: var(--bg-secondary);
      color: var(--text-primary);
      cursor: pointer;
      font-size: 0.875rem;
    }
    .btn:hover { background: var(--border-color); }
    .stats { 
      font-size: 0.875rem; 
      opacity: 0.7;
      background: var(--text-accent);
      color: var(--bg-primary);
      padding: 0.25rem 0.75rem;
      border-radius: 1rem;
    }
    .host-grid { 
      display: grid; 
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); 
      gap: 0.75rem; 
    }
    .host-card { 
      background: var(--bg-secondary); 
      border-radius: 0.5rem; 
      padding: 1rem; 
      cursor: pointer; 
      transition: transform 0.15s, box-shadow 0.15s;
      border: 1px solid transparent;
    }
    .host-card:hover { 
      transform: translateY(-2px); 
      border-color: var(--text-accent);
      box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    }
    .host-card strong { 
      display: block; 
      margin-bottom: 0.5rem;
      word-break: break-all;
    }
    .host-card .meta { 
      opacity: 0.7; 
      font-size: 0.8rem;
      display: flex;
      gap: 0.5rem;
      flex-wrap: wrap;
    }
    .host-card .badge {
      background: var(--bg-primary);
      padding: 0.125rem 0.5rem;
      border-radius: 0.25rem;
    }
    .group-title { 
      color: var(--text-accent); 
      font-size: 1.25rem; 
      margin: 1.5rem 0 0.75rem;
      padding-bottom: 0.5rem;
      border-bottom: 1px solid var(--border-color);
    }
    .group-title:first-child { margin-top: 0; }
    dialog { 
      background: var(--bg-secondary); 
      color: var(--text-primary); 
      border: 1px solid var(--border-color); 
      border-radius: 0.75rem; 
      max-width: 800px; 
      width: 90%;
      padding: 0;
    }
    dialog::backdrop { background: rgba(0,0,0,0.6); }
    .modal-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 1rem 1.5rem;
      border-bottom: 1px solid var(--border-color);
    }
    .modal-header h2 { margin: 0; font-size: 1.25rem; word-break: break-all; }
    .modal-body { padding: 1rem 1.5rem; max-height: 60vh; overflow-y: auto; }
    .modal-footer { padding: 1rem 1.5rem; border-top: 1px solid var(--border-color); text-align: right; }
    pre { 
      background: var(--bg-primary); 
      padding: 1rem; 
      border-radius: 0.5rem; 
      overflow-x: auto;
      font-size: 0.8rem;
      margin: 0;
    }
    .loading { text-align: center; padding: 3rem; opacity: 0.7; }
    .error { 
      background: #5c2b2b; 
      border: 1px solid #8b3a3a;
      padding: 1rem; 
      border-radius: 0.5rem; 
      margin: 1rem 0;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>🌐 Caddy 主机列表</h1>
      <div class="toolbar">
        <span class="stats" id="stats">-/-</span>
        <input type="search" class="search" id="search" placeholder="搜索主机... (ESC清空)">
        <button class="btn" id="refresh" title="刷新数据">🔄</button>
      </div>
    </div>
    <main id="host-list">
      <div class="loading">加载中...</div>
    </main>
  </div>
  
  <dialog id="detail-modal">
    <div class="modal-header">
      <h2 id="modal-title"></h2>
      <button class="btn" onclick="this.closest('dialog').close()">✕</button>
    </div>
    <div class="modal-body">
      <pre id="modal-content"></pre>
    </div>
    <div class="modal-footer">
      <button class="btn" onclick="this.closest('dialog').close()">关闭</button>
    </div>
  </dialog>
  
  <script>
    const DATA_URL = window.location.pathname.replace(/\/$/, '') + '/data';
    let allHosts = [];
    
    async function loadData() {
      try {
        document.getElementById('host-list').innerHTML = '<div class="loading">加载中...</div>';
        const res = await fetch(DATA_URL);
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        allHosts = data.hosts || [];
        render(allHosts);
      } catch (e) {
        document.getElementById('host-list').innerHTML = '<div class="error">加载失败: ' + e.message + '</div>';
        document.getElementById('stats').textContent = '0/0';
      }
    }
    
    function groupByLetter(hosts) {
      const groups = {};
      hosts.forEach(h => {
        const first = h.host.charAt(0).toUpperCase();
        const key = /[A-Z]/.test(first) ? first : '#';
        (groups[key] = groups[key] || []).push(h);
      });
      return Object.keys(groups)
        .sort((a, b) => a === '#' ? 1 : b === '#' ? -1 : a.localeCompare(b))
        .map(letter => ({ 
          letter, 
          hosts: groups[letter].sort((a, b) => a.host.localeCompare(b.host)) 
        }));
    }
    
    function render(hosts) {
      document.getElementById('stats').textContent = hosts.length + '/' + allHosts.length;
      
      if (hosts.length === 0) {
        document.getElementById('host-list').innerHTML = '<div class="loading">无匹配主机</div>';
        return;
      }
      
      const groups = groupByLetter(hosts);
      const html = groups.map(g => ` + "`" + `
        <h2 class="group-title">${g.letter}</h2>
        <div class="host-grid">
          ${g.hosts.map(h => ` + "`" + `
            <div class="host-card" data-host="${escapeHtml(h.host)}" onclick="showDetail(this.dataset.host)">
              <strong>${escapeHtml(h.host)}</strong>
              <div class="meta">
                <span class="badge">${h.routes.length} 路由</span>
                <span class="badge">${h.routes[0]?.listenAddrs?.join(', ') || '-'}</span>
              </div>
            </div>
          ` + "`" + `).join('')}
        </div>
      ` + "`" + `).join('');
      document.getElementById('host-list').innerHTML = html;
    }
    
    function escapeHtml(text) {
      const div = document.createElement('div');
      div.textContent = text;
      return div.innerHTML;
    }
    
    function showDetail(host) {
      const info = allHosts.find(h => h.host === host);
      if (!info) return;
      document.getElementById('modal-title').textContent = host;
      document.getElementById('modal-content').textContent = JSON.stringify(info.routes, null, 2);
      document.getElementById('detail-modal').showModal();
    }
    
    // Search
    const searchInput = document.getElementById('search');
    searchInput.addEventListener('input', () => {
      const keyword = searchInput.value.toLowerCase().trim();
      render(keyword ? allHosts.filter(h => h.host.toLowerCase().includes(keyword)) : allHosts);
    });
    
    // Keyboard shortcuts
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape' && !document.getElementById('detail-modal').open) {
        searchInput.value = '';
        render(allHosts);
        searchInput.focus();
      }
    });
    
    // Refresh button
    document.getElementById('refresh').addEventListener('click', loadData);
    
    // Init
    loadData();
  </script>
</body>
</html>`
}

// WriteDefaultTemplate writes the default template to a file
func WriteDefaultTemplate(path string) error {
	return os.WriteFile(path, []byte(DefaultTemplate()), 0644)
}

// ServeDefaultTemplate creates a handler that serves the default template
func ServeDefaultTemplate(getConfig func() []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/data") {
			configJSON := getConfig()
			hostList := ParseHostList(configJSON)
			response := map[string]any{
				"hosts":     hostList,
				"updatedAt": time.Now().Format(time.RFC3339),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, DefaultTemplate())
	})
}
