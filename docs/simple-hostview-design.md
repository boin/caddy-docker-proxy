# Caddy 主机列表简化方案设计（v2）

## 1. 背景与目标

### 1.1 现状分析

当前 `vendor/index_ui` 是一个完整的 React+Vite 项目，包含：

- **HostView.jsx**: 主机列表展示（按A-Z分组、搜索、详情弹窗）
- **RouteDetails.jsx**: 路由详情展示
- **caddyConfigParser.js**: Caddy 配置解析工具
- **caddyApi.js**: API 调用封装

依赖链：React → Vite → TailwindCSS → DaisyUI → 构建产物

### 1.2 目标

1. **微嵌入**: 最小化修改原项目 Go 代码
2. **可配置**: 通过选项开关和配置 URL 路径
3. **独立模板**: 修改 HTML 不需要重新编译
4. **无外部依赖**: 不依赖 Admin API 的 `/config/` 端点

---

## 2. 技术方案

### 2.1 架构设计

```text
┌─────────────────────────────────────────────────────────────────┐
│                      caddy-docker-proxy                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  DockerLoader                                              │  │
│  │  ├── lastJSONConfig (当前配置)                             │  │
│  │  └── HostStatusHandler (新增)                              │  │
│  │       ├── GET {path}      → 返回 HTML (从模板文件加载)     │  │
│  │       └── GET {path}/data → 返回 JSON (主机列表)           │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
        ┌─────────────────────┐   ┌─────────────────────┐
        │  hostview.html      │   │  Caddy HTTP Server  │
        │  (外部模板文件)      │   │  (复用现有端口)      │
        └─────────────────────┘   └─────────────────────┘
```

### 2.2 配置选项

新增两个配置选项：

| 选项 | 环境变量 | 命令行参数 | 说明 |
|------|----------|------------|------|
| HostStatusURL | `CADDY_DOCKER_HOST_STATUS_URL` | `--host-status-url` | 页面 URL 路径，如 `/caddy/hosts`，留空则禁用 |
| HostStatusTemplate | `CADDY_DOCKER_HOST_STATUS_TEMPLATE` | `--host-status-template` | 模板文件路径，如 `/etc/caddy/hostview.html` |

**示例配置**:

```bash
# 启用主机状态页面
export CADDY_DOCKER_HOST_STATUS_URL="/caddy/hosts"
export CADDY_DOCKER_HOST_STATUS_TEMPLATE="/etc/caddy/hostview.html"
```

### 2.3 文件结构

```text
caddy-docker-proxy/
├── config/
│   └── options.go              # 添加 HostStatusURL, HostStatusTemplate
├── cmd.go                      # 添加命令行参数解析
├── hostview/
│   ├── handler.go              # HTTP handler 实现
│   ├── parser.go               # 主机列表解析逻辑
│   └── template.go             # 模板加载逻辑
├── templates/
│   └── hostview.html           # 默认模板（可选 embed）
└── loader.go                   # 集成 HostStatusHandler
```

### 2.4 URL 端点设计

假设配置 `CADDY_DOCKER_HOST_STATUS_URL=/caddy/hosts`：

| 端点 | 方法 | 返回 | 说明 |
|------|------|------|------|
| `/caddy/hosts` | GET | HTML | 主机列表页面（从模板加载） |
| `/caddy/hosts/data` | GET | JSON | 主机列表数据 |

**JSON 数据格式**:

```json
{
  "hosts": [
    {
      "host": "example.com",
      "routes": [
        {
          "serverName": "srv0",
          "listenAddrs": [":443", ":80"],
          "route": { ... }
        }
      ]
    }
  ],
  "updatedAt": "2024-01-01T12:00:00Z"
}
```

---

## 3. 实现细节

### 3.1 Options 扩展

```go
// config/options.go
type Options struct {
    // ... 现有字段
    HostStatusURL      string  // 主机状态页面 URL，留空禁用
    HostStatusTemplate string  // 模板文件路径
}
```

### 3.2 命令行参数

```go
// cmd.go
fs.String("host-status-url", "",
    "URL path for host status page, e.g. /caddy/hosts. Leave empty to disable.")

fs.String("host-status-template", "",
    "Path to host status HTML template file")
```

### 3.3 HTTP Handler

```go
// hostview/handler.go
package hostview

import (
    "encoding/json"
    "html/template"
    "net/http"
    "sync"
)

type Handler struct {
    templatePath string
    getConfig    func() []byte  // 获取当前配置的函数
    mu           sync.RWMutex
}

func NewHandler(templatePath string, getConfig func() []byte) *Handler {
    return &Handler{
        templatePath: templatePath,
        getConfig:    getConfig,
    }
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 判断是请求页面还是数据
    if strings.HasSuffix(r.URL.Path, "/data") {
        h.serveData(w, r)
    } else {
        h.servePage(w, r)
    }
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request) {
    // 每次请求重新加载模板（支持热更新）
    tmpl, err := template.ParseFiles(h.templatePath)
    if err != nil {
        http.Error(w, "Template error", 500)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    tmpl.Execute(w, nil)
}

func (h *Handler) serveData(w http.ResponseWriter, r *http.Request) {
    configJSON := h.getConfig()
    hostList := ParseHostList(configJSON)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "hosts":     hostList,
        "updatedAt": time.Now().Format(time.RFC3339),
    })
}
```

### 3.4 主机列表解析

```go
// hostview/parser.go
package hostview

import "encoding/json"

type HostInfo struct {
    Host   string      `json:"host"`
    Routes []RouteInfo `json:"routes"`
}

type RouteInfo struct {
    ServerName  string   `json:"serverName"`
    ListenAddrs []string `json:"listenAddrs"`
    Route       any      `json:"route"`
}

func ParseHostList(configJSON []byte) []HostInfo {
    var config map[string]any
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return nil
    }
    
    // 解析逻辑（移植自 caddyConfigParser.js）
    hostMap := make(map[string][]RouteInfo)
    
    apps, _ := config["apps"].(map[string]any)
    http, _ := apps["http"].(map[string]any)
    servers, _ := http["servers"].(map[string]any)
    
    for serverName, serverAny := range servers {
        server, _ := serverAny.(map[string]any)
        listen, _ := server["listen"].([]any)
        routes, _ := server["routes"].([]any)
        
        listenAddrs := make([]string, len(listen))
        for i, l := range listen {
            listenAddrs[i], _ = l.(string)
        }
        
        for _, routeAny := range routes {
            route, _ := routeAny.(map[string]any)
            matches, _ := route["match"].([]any)
            
            for _, matchAny := range matches {
                match, _ := matchAny.(map[string]any)
                hosts, _ := match["host"].([]any)
                
                for _, hostAny := range hosts {
                    host, _ := hostAny.(string)
                    hostMap[host] = append(hostMap[host], RouteInfo{
                        ServerName:  serverName,
                        ListenAddrs: listenAddrs,
                        Route:       route,
                    })
                }
            }
        }
    }
    
    result := make([]HostInfo, 0, len(hostMap))
    for host, routes := range hostMap {
        result = append(result, HostInfo{Host: host, Routes: routes})
    }
    return result
}
```

### 3.5 集成到 DockerLoader

```go
// loader.go
func (dockerLoader *DockerLoader) Start() error {
    // ... 现有代码
    
    // 如果配置了 HostStatusURL，注册 handler
    if dockerLoader.options.HostStatusURL != "" {
        handler := hostview.NewHandler(
            dockerLoader.options.HostStatusTemplate,
            func() []byte { return dockerLoader.lastJSONConfig },
        )
        // 注册到 Caddy 的 HTTP 服务或独立启动
        // 方案见下文
    }
    
    // ... 现有代码
}
```

---

## 4. Handler 注册方案

### 4.1 方案A: 自动注入 Caddyfile 路由（推荐）

在生成 Caddyfile 时，自动添加主机状态页面的路由：

```go
// generator/generator.go
func (g *CaddyfileGenerator) GenerateCaddyfile(log *zap.Logger) ([]byte, []string) {
    // ... 现有代码
    
    // 如果启用了 HostStatus，添加路由
    if g.options.HostStatusURL != "" {
        // 添加类似这样的配置：
        // handle /caddy/hosts* {
        //     host_status {
        //         template /etc/caddy/hostview.html
        //     }
        // }
    }
}
```

需要注册一个 Caddy HTTP handler 模块：

```go
// hostview/caddy_module.go
package hostview

import (
    "github.com/caddyserver/caddy/v2"
    "github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
    caddy.RegisterModule(HostStatus{})
}

type HostStatus struct {
    Template string `json:"template,omitempty"`
}

func (HostStatus) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.handlers.host_status",
        New: func() caddy.Module { return new(HostStatus) },
    }
}

func (h *HostStatus) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
    // 实现逻辑
}
```

### 4.2 方案B: 独立 HTTP 服务器

在单独的端口启动一个 HTTP 服务器：

```go
// 在 loader.go 中
if dockerLoader.options.HostStatusURL != "" {
    go func() {
        handler := hostview.NewHandler(...)
        http.ListenAndServe(":8080", handler)
    }()
}
```

**优点**: 实现简单，与 Caddy 完全解耦
**缺点**: 需要额外端口，可能有防火墙问题

### 4.3 方案对比

| 方案 | 复杂度 | 耦合度 | 端口 | 推荐场景 |
|------|--------|--------|------|----------|
| A: Caddy 模块 | 中 | 低 | 复用 | 生产环境 |
| B: 独立服务 | 低 | 无 | 额外 | 快速验证 |

---

## 5. HTML 模板

### 5.1 模板文件

模板文件放在可配置的路径下，支持热更新：

```html
<!-- /etc/caddy/hostview.html -->
<!DOCTYPE html>
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
    body {
      font-family: system-ui, sans-serif;
      background: var(--bg-primary);
      color: var(--text-primary);
      margin: 0;
      padding: 1rem;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .search { padding: 0.5rem 1rem; border-radius: 0.5rem; border: 1px solid var(--border-color); background: var(--bg-secondary); color: var(--text-primary); }
    .host-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 1rem; }
    .host-card { background: var(--bg-secondary); border-radius: 0.5rem; padding: 1rem; cursor: pointer; transition: transform 0.2s; }
    .host-card:hover { transform: translateY(-2px); }
    .group-title { color: var(--text-accent); font-size: 1.25rem; margin: 1rem 0 0.5rem; }
    dialog { background: var(--bg-secondary); color: var(--text-primary); border: 1px solid var(--border-color); border-radius: 0.5rem; max-width: 800px; width: 90%; }
    dialog::backdrop { background: rgba(0,0,0,0.5); }
    pre { background: var(--bg-primary); padding: 1rem; border-radius: 0.5rem; overflow-x: auto; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>🌐 Caddy 主机列表</h1>
      <input type="search" class="search" id="search" placeholder="搜索主机... (ESC清空)">
    </div>
    <main id="host-list"></main>
  </div>
  
  <dialog id="detail-modal">
    <h2 id="modal-title"></h2>
    <pre id="modal-content"></pre>
    <button onclick="this.closest('dialog').close()">关闭</button>
  </dialog>
  
  <script>
    // 数据端点（相对路径）
    const DATA_URL = window.location.pathname + '/data';
    
    let allHosts = [];
    
    async function loadData() {
      try {
        const res = await fetch(DATA_URL);
        const data = await res.json();
        allHosts = data.hosts || [];
        render(allHosts);
      } catch (e) {
        document.getElementById('host-list').innerHTML = '<p>加载失败: ' + e.message + '</p>';
      }
    }
    
    function groupByLetter(hosts) {
      const groups = {};
      hosts.forEach(h => {
        const key = /^[A-Z]/i.test(h.host) ? h.host[0].toUpperCase() : '#';
        (groups[key] = groups[key] || []).push(h);
      });
      return Object.keys(groups).sort((a, b) => a === '#' ? 1 : b === '#' ? -1 : a.localeCompare(b))
        .map(letter => ({ letter, hosts: groups[letter].sort((a, b) => a.host.localeCompare(b.host)) }));
    }
    
    function render(hosts) {
      const groups = groupByLetter(hosts);
      const html = groups.map(g => `
        <h2 class="group-title">${g.letter}</h2>
        <div class="host-grid">
          ${g.hosts.map(h => `
            <div class="host-card" onclick="showDetail('${h.host}')">
              <strong>${h.host}</strong>
              <div style="opacity:0.7;font-size:0.875rem">
                ${h.routes.length} 路由 | ${h.routes[0]?.listenAddrs?.join(', ') || ''}
              </div>
            </div>
          `).join('')}
        </div>
      `).join('');
      document.getElementById('host-list').innerHTML = html || '<p>无主机数据</p>';
    }
    
    function showDetail(host) {
      const info = allHosts.find(h => h.host === host);
      if (!info) return;
      document.getElementById('modal-title').textContent = host;
      document.getElementById('modal-content').textContent = JSON.stringify(info.routes, null, 2);
      document.getElementById('detail-modal').showModal();
    }
    
    // 搜索
    const searchInput = document.getElementById('search');
    searchInput.addEventListener('input', () => {
      const keyword = searchInput.value.toLowerCase();
      render(allHosts.filter(h => h.host.toLowerCase().includes(keyword)));
    });
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape') { searchInput.value = ''; render(allHosts); searchInput.focus(); }
    });
    
    // 初始化
    loadData();
  </script>
</body>
</html>
```

---

## 6. 实施步骤

### 6.1 Phase 1: 基础实现

1. **修改 `config/options.go`**: 添加 `HostStatusURL` 和 `HostStatusTemplate` 字段
2. **修改 `cmd.go`**: 添加命令行参数和环境变量解析
3. **创建 `hostview/` 目录**: 实现 handler 和 parser

### 6.2 Phase 2: 集成

4. **修改 `loader.go`**: 集成 HostStatusHandler
5. **创建模板文件**: `templates/hostview.html`

### 6.3 Phase 3: 测试

6. **本地测试**: 验证页面和数据端点
7. **Docker 测试**: 验证容器化部署

---

## 7. 与原项目对比

| 维度 | React 版本 | 简化版本 |
|------|-----------|----------|
| 文件大小 | ~300KB (构建后) | ~5KB (模板) |
| Go 代码修改 | 无 | ~200 行 |
| 依赖 | React, Vite, TailwindCSS | 无 |
| 构建 | 需要 npm build | 无需构建 |
| 模板更新 | 需要重新构建 | 热更新 |
| 功能 | 完整 | 核心功能 |

---

## 8. 总结

本方案通过微嵌入方式实现主机状态页面：

- ✅ **可配置**: 通过 `CADDY_DOCKER_HOST_STATUS_URL` 开关和配置
- ✅ **独立模板**: HTML 模板文件可热更新，无需重新编译
- ✅ **无外部依赖**: 直接从 DockerLoader 获取配置，不依赖 Admin API
- ✅ **低耦合**: Go 代码修改约 200 行，集中在新增文件中
- ✅ **复用端口**: 通过 Caddy HTTP handler 模块复用现有端口
